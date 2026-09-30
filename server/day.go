package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Раздел «День»: ежедневный план. Пользователь задаёт свободное время,
// сервер предлагает состав (активные задачи + заметки к повторению,
// вписывающиеся в лимит), метрики в план не входят и время не считают.

// DayItem — позиция сохранённого дня.
type DayItem struct {
	Kind    string `json:"kind"` // task | note
	RefID   int    `json:"ref_id"`
	Title   string `json:"title"`
	Meta    string `json:"meta"` // категория задачи / тема заметки
	Minutes int    `json:"minutes"`
	Done    bool   `json:"done"`
	// SpentMinutes — фактически потраченное время позиции в этот день (0 —
	// не указано). При подсчёте «потрачено» имеет приоритет над Minutes.
	// Для заметок не используется — у них время по символам.
	SpentMinutes int `json:"spent_minutes"`
}

// DayCandidate — кандидат в план из списка предложений.
type DayCandidate struct {
	Kind     string `json:"kind"`
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Meta     string `json:"meta"`
	Minutes  int    `json:"minutes"`
	Selected bool   `json:"selected"`
}

// dayBody — единый ответ для дня.
// completedTask — задача, закрытая в этот день (для отчёта дня).
type completedTask struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	DoneAt   string `json:"done_at"` // RFC3339, UTC
	// CompletedDate — за какой день задача выполнена (ГГГГ-ММ-ДД).
	CompletedDate string `json:"completed_date"`
	// ActualHours — потраченное время задачи (ч) — сумма фактического времени
	// по дням; PlannedHours — плановая оценка (ч).
	ActualHours  float64 `json:"actual_hours"`
	PlannedHours float64 `json:"planned_hours"`
}

type dayBody struct {
	Date          string    `json:"date"`
	BudgetMinutes int       `json:"budget_minutes"`
	TotalMinutes  int       `json:"total_minutes"`
	Items         []DayItem `json:"items"`
	// Report — сохранённый текст отчёта за день; ReportDraft — черновик,
	// который пишется автоматически при вводе (см. «День»).
	Report      string `json:"report"`
	ReportDraft string `json:"report_draft"`
	// CompletedTasks — все задачи, отмеченные выполненными в этот день, даже
	// если их не было в плане. Показываются в отчёте за день.
	CompletedTasks []completedTask `json:"completed_tasks"`
}

// envReadingSpeed — символов в минуту из переменной окружения
// DAY_READING_SPEED; 0, если переменная не задана или некорректна.
func envReadingSpeed() int {
	v := os.Getenv("DAY_READING_SPEED")
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return 0
}

// readingSpeedFor — скорость чтения пользователя из профиля (users.reading_speed).
// 0 или отсутствие значения = «среднее»: сначала DAY_READING_SPEED, иначе 1500.
func readingSpeedFor(username string) int {
	if db != nil && username != "" {
		var v int
		err := db.QueryRow(context.Background(),
			`SELECT reading_speed FROM users WHERE username = $1`, username).Scan(&v)
		if err == nil && v > 0 {
			return v
		}
	}
	if v := envReadingSpeed(); v > 0 {
		return v
	}
	return 1500
}

// readingMinutes — время повторения заметки по количеству символов.
func readingMinutes(content string, speed int) int {
	if speed <= 0 {
		speed = 1500
	}
	chars := len([]rune(content))
	if chars == 0 {
		return 1
	}
	m := int(math.Ceil(float64(chars) / float64(speed)))
	if m < 1 {
		return 1
	}
	return m
}

// taskMinutes — время задачи: planned_hours * 60, минимум 1 минута.
func taskMinutes(hours float64) int {
	m := int(math.Round(hours * 60))
	if m < 1 {
		return 1
	}
	return m
}

// hoursLabel форматирует часы: 2 → "2 ч", 2.5 → "2.5 ч".
func hoursLabel(h float64) string {
	s := strconv.FormatFloat(h, 'f', -1, 64)
	return s + " ч"
}

// repeatIntervals — даты повторений конспекта, отсчитанные в днях от даты его
// создания («первого назначения»). Индекс = число нажатий «Я повторил»:
// 0 — свежая заметка доступна сразу; затем повторение через 1, 2, 4, 7, … дней
// после создания.
var repeatIntervals = []int{0, 1, 2, 4, 7, 14, 30, 60}

// nextRepeatDays — сдвиг (в днях от создания) следующего повторения после
// n выполненных повторений.
func nextRepeatDays(n int) int {
	if n < 0 {
		n = 0
	}
	if n >= len(repeatIntervals) {
		return repeatIntervals[len(repeatIntervals)-1]
	}
	return repeatIntervals[n]
}

// parseNoteTime разбирает время заметки (RFC3339 или ГГГГ-ММ-ДД).
func parseNoteTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}

// noteDueDate — дата следующего повторения конспекта.
// График отсчитывается от даты создания: свежая заметка (0 повторений)
// доступна сразу, дальше +1, +2, +4, +7, … дней. Когда график пройден
// (7+ повторений), интервал 60 дней отсчитывается от последнего повторения.
func noteDueDate(created, updated string, repetitions int) (time.Time, error) {
	anchor := created
	interval := 0
	if repetitions >= len(repeatIntervals) {
		// График пройден — держим интервал 60 дней от последнего повторения.
		anchor = updated
		interval = repeatIntervals[len(repeatIntervals)-1]
	} else {
		interval = repeatIntervals[repetitions]
	}
	t, err := parseNoteTime(anchor)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 0, interval), nil
}

// parseDay нормализует "YYYY-MM-DD"; пусто — сегодня (локальная дата).
func parseDay(s string) (string, error) {
	if s == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", errors.New("некорректная дата (ожидается ГГГГ-ММ-ДД)")
	}
	return s, nil
}

// dueKnowledgeNotes — заметки к повторению на дату day, по возрастанию даты.
// Свежая заметка (0 повторений) доступна сразу после создания, дальше график
// отсчитывается от даты создания. Сравнение идёт по календарной дате, чтобы
// время создания/повторения в течение дня не сдвигало срок.
// Раздел «Знания» общий, поэтому владелец не проверяется.
func dueKnowledgeNotes(day string) []Note {
	dayTime, err := time.Parse("2006-01-02", day)
	if err != nil {
		return nil
	}
	out := make([]Note, 0)
	for _, n := range notes.list() {
		due, err := noteDueDate(n.Created, n.Updated, n.Repetitions)
		if err != nil {
			continue
		}
		// Свежие конспекты (0 повторений) доступны всегда — их можно добавить
		// в день сразу после создания, независимо от даты создания.
		if n.Repetitions == 0 {
			out = append(out, n)
			continue
		}
		// Приводим срок к началу календарного дня в локальной зоне.
		dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, dayTime.Location())
		if dueDay.After(dayTime) {
			continue // срок ещё не наступил
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		di, _ := noteDueDate(out[i].Created, out[i].Updated, out[i].Repetitions)
		dj, _ := noteDueDate(out[j].Created, out[j].Updated, out[j].Repetitions)
		// Сравниваем по календарной дате срока.
		diD := time.Date(di.Year(), di.Month(), di.Day(), 0, 0, 0, 0, dayTime.Location())
		djD := time.Date(dj.Year(), dj.Month(), dj.Day(), 0, 0, 0, 0, dayTime.Location())
		if !diD.Equal(djD) {
			return diD.Before(djD)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// handleDaySuggest — предложения состава дня под лимит времени.
func handleDaySuggest(c *gin.Context) {
	var req struct {
		Date    string `json:"date"`
		Minutes int    `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Minutes < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите доступное время"})
		return
	}

	sessData, _ := c.MustGet("session").(session)
	speed := readingSpeedFor(sessData.username)

	// Активные задачи: по дедлайну (без дедлайна — в конец), затем старые.
	type tc struct {
		task Task
		cand DayCandidate
	}
	rows := make([]tc, 0)
	for _, t := range tasks.list(sessData.username) {
		if t.Status != taskTodo && t.Status != taskInProgress {
			continue
		}
		rows = append(rows, tc{task: t, cand: DayCandidate{
			Kind: "task", ID: t.ID, Title: t.Title,
			Meta:    t.Category + " · " + hoursLabel(t.PlannedHours),
			Minutes: taskMinutes(t.PlannedHours),
		}})
	}
	sort.Slice(rows, func(i, j int) bool {
		di, dj := rows[i].task.Deadline, rows[j].task.Deadline
		if (di == "") != (dj == "") {
			return di != "" // задачи с дедлайном раньше
		}
		if di != dj {
			return di < dj
		}
		return rows[i].task.ID < rows[j].task.ID
	})
	taskCands := make([]DayCandidate, 0, len(rows))
	for _, r := range rows {
		taskCands = append(taskCands, r.cand)
	}

	// Заметки к повторению: время по символам.
	noteCands := make([]DayCandidate, 0)
	dueNotes := dueKnowledgeNotes(day)
	for _, n := range dueNotes {
		noteCands = append(noteCands, DayCandidate{
			Kind: "note", ID: n.ID, Title: n.Title, Meta: n.Topic,
			Minutes: readingMinutes(n.Content, speed),
		})
	}

	// Диагностика: конспекты есть, но к повторению ничего не отобралось.
	if len(dueNotes) == 0 && len(notes.list()) > 0 {
		var sb strings.Builder
		for _, n := range notes.list() {
			due, err := noteDueDate(n.Created, n.Updated, n.Repetitions)
			dueStr := "?"
			if err == nil {
				dueStr = due.Format(time.RFC3339)
			}
			fmt.Fprintf(&sb, " [id=%d reps=%d created=%s due=%s]",
				n.ID, n.Repetitions, n.Created, dueStr)
		}
		log.Printf("DAY: на %s нет заметок к повторению, всего %d:%s",
			day, len(notes.list()), sb.String())
	}

	// Подбор: сначала задачи, потом заметки — пока влезают в лимит.
	left := req.Minutes
	for i := range taskCands {
		if left <= 0 {
			break
		}
		if taskCands[i].Minutes <= left {
			taskCands[i].Selected = true
			left -= taskCands[i].Minutes
		}
	}
	for i := range noteCands {
		if left <= 0 {
			break
		}
		if noteCands[i].Minutes <= left {
			noteCands[i].Selected = true
			left -= noteCands[i].Minutes
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"date":             day,
		"minutes":          req.Minutes,
		"used_minutes":     req.Minutes - left,
		"reading_speed":    speed,
		"tasks":            taskCands,
		"notes":            noteCands,
		"metrics_included": true, // метрики всегда на месте, время не считают
	})
}

// dayItemTitle возвращает заголовок и мета-подпись позиции по kind/ref.
func dayItemTitle(username, kind string, refID int) (title, meta string, ok bool) {
	switch kind {
	case "task":
		t, found := tasks.getOwned(username, refID)
		if !found {
			return "", "", false
		}
		return t.Title, t.Category, true
	case "note":
		n, found := notes.get(refID)
		if !found {
			return "", "", false
		}
		return n.Title, n.Topic, true
	}
	return "", "", false
}

// resolveItemMinutes — минуты для позиции: для задач planned*60, для заметок
// время чтения по символам (используется при сохранении плана).
func resolveItemMinutes(username, kind string, refID int) (int, bool) {
	switch kind {
	case "task":
		t, found := tasks.getOwned(username, refID)
		if !found {
			return 0, false
		}
		return taskMinutes(t.PlannedHours), true
	case "note":
		n, found := notes.get(refID)
		if !found {
			return 0, false
		}
		return readingMinutes(n.Content, readingSpeedFor(username)), true
	}
	return 0, false
}

// loadDayItems собирает план из БД по дате.
func loadDayItems(username, day string) (dayBody, bool) {
	if db == nil {
		return dayBody{}, false
	}
	ctx := context.Background()
	var body dayBody
	body.Date = day
	body.Items = make([]DayItem, 0)
	// Задачи, закрытые в этот день: не зависят от того, был ли план.
	body.CompletedTasks = dayCompletedTasks(ctx, username, day)

	var planID int
	err := db.QueryRow(ctx,
		`SELECT id, budget_minutes, COALESCE(report, ''), COALESCE(report_draft, '')
		 FROM day_plans
		 WHERE username = $1 AND day = $2`, username, day).
		Scan(&planID, &body.BudgetMinutes, &body.Report, &body.ReportDraft)
	if err != nil {
		return body, false
	}

	items := make([]DayItem, 0)
	rows, err := db.Query(ctx,
		`SELECT kind, ref_id, minutes, done, position, actual_minutes
		 FROM day_items WHERE plan_id = $1 ORDER BY position`, planID)
	if err != nil {
		return body, false
	}
	defer rows.Close()
	for rows.Next() {
		var it DayItem
		var pos int
		if err := rows.Scan(&it.Kind, &it.RefID, &it.Minutes, &it.Done, &pos, &it.SpentMinutes); err != nil {
			continue
		}
		title, meta, ok := dayItemTitle(username, it.Kind, it.RefID)
		if !ok {
			continue // задача/заметка удалены — позицию пропускаем
		}
		it.Title, it.Meta = title, meta
		// Задача, закрытая в разделе «Задачи», тоже считается выполненной
		// позицией дня: иначе строка не подсвечивалась бы зелёным, а из списка
		// «выполнено вне плана» она исключена (она уже есть в плане).
		if it.Kind == "task" && !it.Done {
			if t, found := tasks.getOwned(username, it.RefID); found && t.Status == taskDone {
				it.Done = true
			}
		}
		items = append(items, it)
		body.TotalMinutes += it.Minutes
	}
	body.Items = items
	return body, true
}

// dayCompletedTasks возвращает задачи пользователя, закрытые в указанный день.
// День берётся из completed_date — локальной даты, которую пользователь может
// задать (в том числе задним числом), поэтому часовой пояс здесь не нужен.
// ActualHours — сумма фактического времени задачи по дням (0, если фактов нет).
func dayCompletedTasks(ctx context.Context, username, day string) []completedTask {
	out := make([]completedTask, 0)
	rows, err := db.Query(ctx,
		`SELECT t.id, t.title, t.category,
		        to_char(t.completed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        COALESCE(to_char(t.completed_date,'YYYY-MM-DD'),''),
		        COALESCE((SELECT SUM(i.actual_minutes)
		                    FROM day_items i
		                    JOIN day_plans p ON p.id = i.plan_id
		                   WHERE i.kind = 'task' AND i.ref_id = t.id
		                     AND p.username = t.username), 0)::float8 / 60.0,
		        t.planned_hours
		   FROM tasks t
		  WHERE t.username = $1 AND t.status = 'done' AND t.completed_date = $2::date
		  ORDER BY t.completed_at`,
		username, day)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t completedTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.DoneAt, &t.CompletedDate, &t.ActualHours, &t.PlannedHours); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// handleGetDay — план на дату (или пустой, если день ещё не сформирован).
func handleGetDay(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	day, err := parseDay(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body, found := loadDayItems(sessData.username, day)
	if !found {
		// Плана может не быть, но закрытые в этот день задачи уже собраны.
		body.Date = day
	}
	c.JSON(http.StatusOK, body)
}

// handleSaveDay сохраняет (перезаписывает) план дня.
// Тело: {"date": "ГГГГ-ММ-ДД", "budget_minutes": число, "items":[{"kind":"task|note","ref_id":N}]}
func handleSaveDay(c *gin.Context) {
	var req struct {
		Date          string `json:"date"`
		BudgetMinutes int    `json:"budget_minutes"`
		Items         []struct {
			Kind  string `json:"kind"`
			RefID int    `json:"ref_id"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.BudgetMinutes < 0 {
		req.BudgetMinutes = 0
	}

	sessData, _ := c.MustGet("session").(session)
	username := sessData.username
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "БД недоступна"})
		return
	}
	defer tx.Rollback(ctx)

	var planID int
	err = tx.QueryRow(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (username, day)
		 DO UPDATE SET budget_minutes = EXCLUDED.budget_minutes, updated = now()
		 RETURNING id`,
		username, day, req.BudgetMinutes).Scan(&planID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить день: " + err.Error()})
		return
	}

	type kv struct {
		kind string
		id   int
	}
	// Сохраняем факт и отметку «выполнено» у позиций, которые остаются в плане:
	// пере-формирование дня не должно обнулять уже введённое время.
	type prevItem struct {
		actual int
		done   bool
	}
	prev := make(map[kv]prevItem)
	if rows, err := tx.Query(ctx,
		`SELECT kind, ref_id, actual_minutes, done FROM day_items WHERE plan_id = $1`, planID); err == nil {
		for rows.Next() {
			var k kv
			var p prevItem
			if err := rows.Scan(&k.kind, &k.id, &p.actual, &p.done); err == nil {
				prev[k] = p
			}
		}
		rows.Close()
	}

	if _, err := tx.Exec(ctx, `DELETE FROM day_items WHERE plan_id = $1`, planID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить день: " + err.Error()})
		return
	}

	seen := make(map[kv]bool)
	for pos, it := range req.Items {
		if it.Kind != "task" && it.Kind != "note" {
			continue
		}
		k := kv{it.Kind, it.RefID}
		if seen[k] {
			continue
		}
		minutes, ok := resolveItemMinutes(username, it.Kind, it.RefID)
		if !ok {
			continue
		}
		seen[k] = true
		p := prev[k]
		if _, err := tx.Exec(ctx,
			`INSERT INTO day_items (plan_id, kind, ref_id, minutes, done, position, actual_minutes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			planID, it.Kind, it.RefID, minutes, p.done, pos, p.actual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить день: " + err.Error()})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить день: " + err.Error()})
		return
	}

	body, _ := loadDayItems(username, day)
	c.JSON(http.StatusOK, body)
}

// handleSetDayItemDone — отметить позицию плана дня выполненной (done=true)
// или снять отметку. Тело: {"date", "kind", "ref_id", "done"}.
func handleSetDayItemDone(c *gin.Context) {
	var req struct {
		Date  string `json:"date"`
		Kind  string `json:"kind"`
		RefID int    `json:"ref_id"`
		Done  bool   `json:"done"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Kind != "task" && req.Kind != "note" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный тип позиции"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	_, err = db.Exec(context.Background(),
		`UPDATE day_items SET done = $1
		 WHERE kind = $2 AND ref_id = $3 AND plan_id =
		       (SELECT id FROM day_plans WHERE username = $4 AND day = $5)`,
		req.Done, req.Kind, req.RefID, sessData.username, day)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить позицию дня"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// maxDayItemSpentMinutes — верхняя граница фактического времени по позиции дня
// (сутки): защита от случайного ввода.
const maxDayItemSpentMinutes = 24 * 60

// handleSetDayItemSpent — фактически потраченное время позиции дня
// (PUT /api/day/spent, тело {"date","kind","ref_id","minutes"}).
// Пишется в day_items.actual_minutes; 0 очищает значение. Только для задач:
// у конспектов время считается по символам.
// Если задачи ещё нет в плане этого дня, она добавляется в день (план при
// необходимости создаётся) — чтобы время можно было указать прямо из задачи.
func handleSetDayItemSpent(c *gin.Context) {
	var req struct {
		Date    string `json:"date"`
		Kind    string `json:"kind"`
		RefID   int    `json:"ref_id"`
		Minutes int    `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if req.Kind != "task" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Факт можно указать только для задачи"})
		return
	}
	if req.Minutes < 0 || req.Minutes > maxDayItemSpentMinutes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректное время"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	username := sessData.username
	// Чужая/несуществующая задача в день не добавляется.
	if _, ok := tasks.getOwned(username, req.RefID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена"})
		return
	}

	ctx := context.Background()
	// План дня создаём при необходимости (шапку «доступное время» пользователь
	// задаст сам при формировании).
	if _, err := db.Exec(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, 0)
		 ON CONFLICT (username, day) DO NOTHING`, username, day); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить время"})
		return
	}
	var planID int
	if err := db.QueryRow(ctx,
		`SELECT id FROM day_plans WHERE username = $1 AND day = $2`,
		username, day).Scan(&planID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить время"})
		return
	}

	// Позиция дня создаётся при необходимости: задача с указанным временем
	// должна появиться в «Плане дня». Плановые минуты — обычная оценка задачи.
	minutes, _ := resolveItemMinutes(username, "task", req.RefID)
	if _, err := db.Exec(ctx,
		`INSERT INTO day_items (plan_id, kind, ref_id, minutes, position, actual_minutes)
		 VALUES ($1, 'task', $2, $3,
		         COALESCE((SELECT MAX(position) + 1 FROM day_items WHERE plan_id = $1), 0),
		         $4)
		 ON CONFLICT (plan_id, kind, ref_id)
		 DO UPDATE SET actual_minutes = EXCLUDED.actual_minutes`,
		planID, req.RefID, minutes, req.Minutes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить время"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "minutes": req.Minutes})
}

// DaySummary — строка истории (прошедшие дни).
type DaySummary struct {
	Date          string `json:"date"`
	BudgetMinutes int    `json:"budget_minutes"`
	TotalMinutes  int    `json:"total_minutes"`
	Tasks         int    `json:"tasks"`
	Notes         int    `json:"notes"`
	// SpentMinutes — фактически проставленное время дня по задачам: факт дня,
	// иначе факт задачи (план и повторения не считаются). Без времени чтения.
	SpentMinutes int `json:"spent_minutes"`
	// HasPlan — был ли сохранён план на этот день. День может попасть в
	// историю только из-за закрытых задач (тогда плана нет).
	HasPlan bool `json:"has_plan"`
}

// handleDayHistory — список дней (сначала новые): сохранённые планы и дни,
// в которые были закрыты задачи. День задачи берётся из completed_date —
// локальной даты, которую пользователь может задать (в том числе задним числом).
func handleDayHistory(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"days": []DaySummary{}})
		return
	}
	rows, err := db.Query(context.Background(),
		`WITH plan_days AS (
		     SELECT p.day AS day,
		            p.budget_minutes,
		            COALESCE(SUM(i.minutes), 0) AS total_minutes,
		            COUNT(*) FILTER (WHERE i.kind = 'task') AS tasks,
		            COUNT(*) FILTER (WHERE i.kind = 'note') AS notes
		     FROM day_plans p
		     LEFT JOIN day_items i ON i.plan_id = p.id
		     WHERE p.username = $1
		     GROUP BY p.id, p.day, p.budget_minutes
		 ),
		 done_days AS (
		     SELECT t.completed_date AS day
		     FROM tasks t
		     WHERE t.username = $1 AND t.status = 'done' AND t.completed_date IS NOT NULL
		     GROUP BY 1
		 ),
		 /* Фактически проставленное время по задачам дня: сумма фактов позиций
		    дня (day_items.actual_minutes). Плановые оценки не считаем, а время
		    задачи, закрытой вне плана, сюда не попадает — для неё факта нет. */
		 spent_days AS (
		     SELECT p.day AS day,
		            COALESCE(SUM(i.actual_minutes), 0)::int AS task_minutes
		       FROM day_plans p
		       JOIN day_items i ON i.plan_id = p.id AND i.kind = 'task'
		      WHERE p.username = $1
		      GROUP BY p.day
		 )
		 SELECT to_char(d.day, 'YYYY-MM-DD'),
		        COALESCE(pd.budget_minutes, 0),
		        COALESCE(pd.total_minutes, 0),
		        COALESCE(pd.tasks, 0),
		        COALESCE(pd.notes, 0),
		        COALESCE(sd.task_minutes, 0),
		        (pd.day IS NOT NULL)
		 FROM (SELECT day FROM plan_days UNION SELECT day FROM done_days) d
		 LEFT JOIN plan_days pd ON pd.day = d.day
		 LEFT JOIN spent_days sd ON sd.day = d.day
		 ORDER BY d.day DESC
		 LIMIT 90`, sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить историю"})
		return
	}
	defer rows.Close()
	days := make([]DaySummary, 0)
	for rows.Next() {
		var d DaySummary
		if err := rows.Scan(&d.Date, &d.BudgetMinutes, &d.TotalMinutes, &d.Tasks, &d.Notes, &d.SpentMinutes, &d.HasPlan); err == nil {
			days = append(days, d)
		}
	}
	c.JSON(http.StatusOK, gin.H{"days": days})
}

// parseDayInt — вспомогательная проверка для времени (оставлено для будущего
// использования в UI-хелперах).
func parseDayInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// dayFmtMinutes — минуты → "2 ч 15 мин" (для ответов/подписей).
func dayFmtMinutes(m int) string {
	if m < 60 {
		return fmt.Sprintf("%d мин", m)
	}
	h := m / 60
	r := m % 60
	if r == 0 {
		return fmt.Sprintf("%d ч", h)
	}
	return fmt.Sprintf("%d ч %d мин", h, r)
}
