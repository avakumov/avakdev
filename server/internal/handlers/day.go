package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/store"
)

// Раздел «День»: ежедневный план. Пользователь задаёт свободное время,
// сервер предлагает состав (активные задачи + заметки к повторению,
// вписывающиеся в лимит), метрики в план не входят и время не считают.

// dayItem — позиция сохранённого дня (определение живёт в store).
type dayItem = store.DayItem

// dayCandidate — кандидат в план из списка предложений.
type dayCandidate struct {
	Kind     string `json:"kind"`
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Meta     string `json:"meta"`
	Minutes  int    `json:"minutes"`
	Selected bool   `json:"selected"`
}

// completedTask / dayBody — определения живут в store (день, его строки и
// закрытые в этот день задачи).
type completedTask = store.CompletedTask

type dayBody = store.DayBody

// DaySuggest — предложения состава дня под лимит времени.
func (h *Handlers) DaySuggest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date    string `json:"date"`
		Minutes int    `json:"minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Minutes < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Укажите доступное время"})
		return
	}

	sessData, _ := sessionOf(r)
	speed := h.App.ReadingSpeed(sessData.Username)

	// Активные задачи: по дедлайну (без дедлайна — в конец), затем старые.
	type tc struct {
		task app.Task
		cand dayCandidate
	}
	rows := make([]tc, 0)
	for _, t := range h.App.Tasks.List(sessData.Username) {
		if t.Status != app.TaskTodo && t.Status != app.TaskInProgress {
			continue
		}
		rows = append(rows, tc{task: t, cand: dayCandidate{
			Kind: "task", ID: t.ID, Title: t.Title,
			Meta:    t.Category + " · " + app.HoursLabel(t.PlannedHours),
			Minutes: app.TaskMinutes(t.PlannedHours),
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
	taskCands := make([]dayCandidate, 0, len(rows))
	for _, r := range rows {
		taskCands = append(taskCands, r.cand)
	}

	// Заметки к повторению: время по символам.
	noteCands := make([]dayCandidate, 0)
	dueNotes := h.App.DueKnowledgeNotes(day)
	for _, n := range dueNotes {
		noteCands = append(noteCands, dayCandidate{
			Kind: "note", ID: n.ID, Title: n.Title, Meta: n.Topic,
			Minutes: app.ReadingMinutes(n.Content, speed),
		})
	}

	// Диагностика: конспекты есть, но к повторению ничего не отобралось.
	if len(dueNotes) == 0 && len(h.App.Knowledge.List()) > 0 {
		var sb strings.Builder
		for _, n := range h.App.Knowledge.List() {
			due, err := app.NoteDueDate(n.Created, n.Updated, n.Repetitions)
			dueStr := "?"
			if err == nil {
				dueStr = due.Format(time.RFC3339)
			}
			fmt.Fprintf(&sb, " [id=%d reps=%d created=%s due=%s]",
				n.ID, n.Repetitions, n.Created, dueStr)
		}
		log.Printf("DAY: на %s нет заметок к повторению, всего %d:%s",
			day, len(h.App.Knowledge.List()), sb.String())
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

	writeJSON(w, http.StatusOK, map[string]any{
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
func (h *Handlers) dayItemTitle(username, kind string, refID int) (title, meta string, ok bool) {
	switch kind {
	case "task":
		t, found := h.App.Tasks.GetOwned(username, refID)
		if !found {
			return "", "", false
		}
		return t.Title, t.Category, true
	case "note":
		n, found := h.App.Knowledge.Get(refID)
		if !found {
			return "", "", false
		}
		return n.Title, n.Topic, true
	}
	return "", "", false
}

// resolveItemMinutes — минуты для позиции: для задач planned*60, для заметок
// время чтения по символам (используется при сохранении плана).
func (h *Handlers) resolveItemMinutes(username, kind string, refID int) (int, bool) {
	switch kind {
	case "task":
		t, found := h.App.Tasks.GetOwned(username, refID)
		if !found {
			return 0, false
		}
		return app.TaskMinutes(t.PlannedHours), true
	case "note":
		n, found := h.App.Knowledge.Get(refID)
		if !found {
			return 0, false
		}
		return app.ReadingMinutes(n.Content, h.App.ReadingSpeed(username)), true
	}
	return 0, false
}

// loadDayItems собирает план из БД по дате (SQL — в store.Day).
func (h *Handlers) loadDayItems(username, day string) (dayBody, bool) {
	if h.App.DB == nil {
		return dayBody{}, false
	}
	ctx := context.Background()
	var body dayBody
	body.Date = day
	body.Items = make([]dayItem, 0)
	// Задачи, закрытые в этот день: не зависят от того, был ли план.
	body.CompletedTasks = h.App.Day.CompletedTasks(ctx, username, day)

	planID, budget, report, ok := h.App.Day.PlanInfo(ctx, username, day)
	if !ok {
		return body, false
	}
	body.BudgetMinutes, body.Report = budget, report

	raw, err := h.App.Day.Items(ctx, planID)
	if err != nil {
		return body, false
	}
	items := make([]dayItem, 0, len(raw))
	for _, it := range raw {
		title, meta, ok := h.dayItemTitle(username, it.Kind, it.RefID)
		if !ok {
			continue // задача/заметка удалены — позицию пропускаем
		}
		it.Title, it.Meta = title, meta
		// Задача, закрытая в разделе «Задачи», тоже считается выполненной
		// позицией дня: иначе строка не подсвечивалась бы зелёным, а из списка
		// «выполнено вне плана» она исключена (она уже есть в плане).
		if it.Kind == "task" && !it.Done {
			if t, found := h.App.Tasks.GetOwned(username, it.RefID); found && t.Status == app.TaskDone {
				it.Done = true
			}
		}
		items = append(items, it)
		body.TotalMinutes += it.Minutes
	}
	body.Items = items
	return body, true
}

// GetDay — план на дату (или пустой, если день ещё не сформирован).
func (h *Handlers) GetDay(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	day, err := app.ParseDay(r.URL.Query().Get("date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	body, found := h.loadDayItems(sessData.Username, day)
	if !found {
		// Плана может не быть, но закрытые в этот день задачи уже собраны.
		body.Date = day
	}
	writeJSON(w, http.StatusOK, body)
}

// SaveDay сохраняет (перезаписывает) план дня.
// Тело: {"date": "ГГГГ-ММ-ДД", "budget_minutes": число, "items":[{"kind":"task|note","ref_id":N}]}
func (h *Handlers) SaveDay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date          string `json:"date"`
		BudgetMinutes int    `json:"budget_minutes"`
		Items         []struct {
			Kind  string `json:"kind"`
			RefID int    `json:"ref_id"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.BudgetMinutes < 0 {
		req.BudgetMinutes = 0
	}

	sessData, _ := sessionOf(r)
	username := sessData.Username
	ctx := context.Background()

	type kv struct {
		kind string
		id   int
	}
	seen := make(map[kv]bool)
	items := make([]store.SaveItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Kind != "task" && it.Kind != "note" {
			continue
		}
		k := kv{it.Kind, it.RefID}
		if seen[k] {
			continue
		}
		minutes, ok := h.resolveItemMinutes(username, it.Kind, it.RefID)
		if !ok {
			continue
		}
		seen[k] = true
		items = append(items, store.SaveItem{Kind: it.Kind, RefID: it.RefID, Minutes: minutes})
	}

	if err := h.App.Day.SavePlan(ctx, username, day, req.BudgetMinutes, items); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить день: " + err.Error()})
		return
	}

	body, _ := h.loadDayItems(username, day)
	writeJSON(w, http.StatusOK, body)
}

// SetDayItemDone — отметить позицию плана дня выполненной (done=true)
// или снять отметку. Тело: {"date", "kind", "ref_id", "done"}.
func (h *Handlers) SetDayItemDone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date  string `json:"date"`
		Kind  string `json:"kind"`
		RefID int    `json:"ref_id"`
		Done  bool   `json:"done"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Kind != "task" && req.Kind != "note" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный тип позиции"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.Day.SetItemDone(context.Background(), sessData.Username, day, req.Kind, req.RefID, req.Done); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось обновить позицию дня"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// maxDayItemSpentMinutes — верхняя граница фактического времени по позиции дня
// (сутки): защита от случайного ввода.
const maxDayItemSpentMinutes = 24 * 60

// SetDayItemSpent — фактически потраченное время позиции дня
// (PUT /api/day/spent, тело {"date","kind","ref_id","minutes"}).
// Пишется в day_items.actual_minutes; 0 очищает значение. Только для задач:
// у конспектов время считается по символам.
// Если задачи ещё нет в плане этого дня, она добавляется в день (план при
// необходимости создаётся) — чтобы время можно было указать прямо из задачи.
func (h *Handlers) SetDayItemSpent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date    string `json:"date"`
		Kind    string `json:"kind"`
		RefID   int    `json:"ref_id"`
		Minutes int    `json:"minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if req.Kind != "task" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Факт можно указать только для задачи"})
		return
	}
	if req.Minutes < 0 || req.Minutes > maxDayItemSpentMinutes {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректное время"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)
	username := sessData.Username
	// Чужая/несуществующая задача в день не добавляется.
	if _, ok := h.App.Tasks.GetOwned(username, req.RefID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Задача не найдена"})
		return
	}

	ctx := context.Background()
	// План дня создаём при необходимости (шапку «доступное время» пользователь
	// задаст сам при формировании).
	planID, err := h.App.Day.EnsurePlan(ctx, username, day)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить время"})
		return
	}

	// Позиция дня создаётся при необходимости: задача с указанным временем
	// должна появиться в «Плане дня». Плановые минуты — обычная оценка задачи.
	minutes, _ := h.resolveItemMinutes(username, "task", req.RefID)
	if err := h.App.Day.SetItemSpent(ctx, planID, req.RefID, req.Minutes, minutes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить время"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "minutes": req.Minutes})
}

// daySummary — строка истории (определение живёт в store).
type daySummary = store.DaySummary

// DayHistory — список дней (сначала новые): сохранённые планы и дни,
// в которые были закрыты задачи. День задачи берётся из completed_date —
// локальной даты, которую пользователь может задать (в том числе задним числом).
func (h *Handlers) DayHistory(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	if h.App.DB == nil {
		writeJSON(w, http.StatusOK, map[string]any{"days": []daySummary{}})
		return
	}
	days, err := h.App.Day.History(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить историю"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": days})
}
