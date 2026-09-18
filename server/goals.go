package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Статусы целей.
const (
	goalActive    = "active"
	goalPaused    = "paused"
	goalAchieved  = "achieved"
	goalCancelled = "cancelled"
)

var validGoalStatuses = map[string]bool{
	goalActive:    true,
	goalPaused:    true,
	goalAchieved:  true,
	goalCancelled: true,
}

// goalTaskDraft — черновик задачи, который пользователь получил от ИИ или
// добавил при создании цели. Сохраняется вместе с целью в момент создания.
type goalTaskDraft struct {
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Category     string  `json:"category"`
	PlannedHours float64 `json:"planned_hours"`
}

// Goal — цель раздела «Цели»: результат с дедлайном и статусом.
// Progress в БД не хранится и вычисляется на лету из привязанных задач:
// доля выполненных задач среди неотменённых.
type Goal struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetDate  string `json:"target_date"` // YYYY-MM-DD или пусто
	Status      string `json:"status"`
	Progress    int    `json:"progress"` // 0..100, вычисляется при ответе
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// goalStore — хранилище целей.
type goalStore struct {
	mu     sync.Mutex
	data   map[int]Goal
	nextID int
	hasDB  bool
}

var goals *goalStore

// initGoals инициализирует хранилище целей и подгружает их из БД.
func initGoals() error {
	goals = &goalStore{
		data:   make(map[int]Goal),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !goals.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        title,
		        description,
		        COALESCE(to_char(target_date,'YYYY-MM-DD'),''),
		        status,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM goals`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Username, &g.Title, &g.Description,
			&g.TargetDate, &g.Status, &g.Created, &g.Updated); err != nil {
			return err
		}
		goals.data[g.ID] = g
		if g.ID >= goals.nextID {
			goals.nextID = g.ID + 1
		}
	}
	return rows.Err()
}

// list возвращает цели пользователя, новые сверху.
func (s *goalStore) list(username string) []Goal {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Goal, 0)
	for _, g := range s.data {
		if g.Username == username {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// getOwned возвращает цель, если она принадлежит пользователю.
func (s *goalStore) getOwned(username string, id int) (Goal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, false
	}
	return g, true
}

// create добавляет новую цель.
func (s *goalStore) create(username, title, description, targetDate, status string) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("укажите название цели")
	}
	if !validGoalStatuses[status] {
		return Goal{}, errors.New("некорректный статус цели")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	g := Goal{
		Username:    username,
		Title:       title,
		Description: description,
		TargetDate:  targetDate,
		Status:      status,
		Created:     now,
		Updated:     now,
	}

	if s.hasDB {
		var dl interface{}
		if targetDate != "" {
			dl = targetDate
		}
		err := db.QueryRow(context.Background(),
			`INSERT INTO goals (username, title, description, target_date, status)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, title, description, dl, status).
			Scan(&g.ID, &g.Created, &g.Updated)
		if err != nil {
			return Goal{}, err
		}
		if g.ID >= s.nextID {
			s.nextID = g.ID + 1
		}
	} else {
		g.ID = s.nextID
		s.nextID++
	}

	s.data[g.ID] = g
	return g, nil
}

// update обновляет цель.
func (s *goalStore) update(username string, id int, title, description, targetDate, status string) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("укажите название цели")
	}
	if !validGoalStatuses[status] {
		return Goal{}, errors.New("некорректный статус цели")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, errors.New("цель не найдена")
	}

	g.Title = title
	g.Description = description
	g.TargetDate = targetDate
	g.Status = status
	g.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.hasDB {
		var dl interface{}
		if targetDate != "" {
			dl = targetDate
		}
		if _, err := db.Exec(context.Background(),
			`UPDATE goals
			 SET title = $2, description = $3, target_date = $4,
			     status = $5, updated = now()
			 WHERE id = $1`,
			id, g.Title, g.Description, dl, g.Status); err != nil {
			return Goal{}, err
		}
	}

	s.data[id] = g
	return g, nil
}

// completionStats возвращает, сколько задач, привязанных к цели, выполнено
// (done) и сколько «активны» — участвуют в расчёте прогресса. Отменённые
// (cancelled) задачи не учитываются вовсе.
func (s *taskStore) completionStats(username string, goalID int) (done, active int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.data {
		if t.Username != username || t.GoalID == nil || *t.GoalID != goalID {
			continue
		}
		if t.Status == taskCancelled {
			continue
		}
		active++
		if t.Status == taskDone {
			done++
		}
	}
	return done, active
}

// clearGoalLinks убирает у задач пользователя ссылку на удаляемую цель.
// В БД это делает внешний ключ (ON DELETE SET NULL), здесь — в памяти сервера.
func (s *taskStore) clearGoalLinks(username string, goalID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.data {
		if t.Username == username && t.GoalID != nil && *t.GoalID == goalID {
			t.GoalID = nil
			s.data[id] = t
		}
	}
}

// removeByGoal удаляет все задачи пользователя, привязанные к цели
// (и из БД, и из памяти). Используется при удалении цели с опцией
// «удалить привязанные задачи». Возвращает количество удалённых задач.
func (s *taskStore) removeByGoal(username string, goalID int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]int, 0)
	for id, t := range s.data {
		if t.Username == username && t.GoalID != nil && *t.GoalID == goalID {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM tasks WHERE username = $1 AND goal_id = $2`,
			username, goalID); err != nil {
			return 0, err
		}
	}
	for _, id := range ids {
		delete(s.data, id)
	}
	return len(ids), nil
}

// delete удаляет цель пользователя. При deleteTasks=true привязанные задачи
// удаляются вместе с целью; иначе задачи остаются, но ссылка на цель
// сбрасывается (в БД — внешним ключом ON DELETE SET NULL, в памяти — вручную).
func (s *goalStore) delete(username string, id int, deleteTasks bool) error {
	s.mu.Lock()
	g, ok := s.data[id]
	if !ok || g.Username != username {
		s.mu.Unlock()
		return errors.New("цель не найдена")
	}
	// Не держим блокировку целей во время обращения к задачам: задачи сначала
	// удаляются (пока ссылка ещё стоит), затем цель.
	s.mu.Unlock()

	if tasks != nil {
		if deleteTasks {
			if _, err := tasks.removeByGoal(username, id); err != nil {
				return err
			}
		} else {
			tasks.clearGoalLinks(username, id)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok || s.data[id].Username != username {
		return errors.New("цель не найдена")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM goals WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// computeProgress заполняет Progress цели на лету по привязанным задачам:
// процент выполненных задач среди неотменённых. Значение не хранится в БД.
// 100% достижимо только при статусе «достигнута» (achieved): даже если все
// задачи выполнены, пока цель официально не завершена, показывается 99%.
func (g *Goal) computeProgress() {
	if g.Status == goalAchieved {
		g.Progress = 100
		return
	}
	if tasks == nil {
		g.Progress = 0
		return
	}
	done, active := tasks.completionStats(g.Username, g.ID)
	if active <= 0 {
		g.Progress = 0
		return
	}
	p := int(math.Round(float64(done) * 100 / float64(active)))
	if p >= 100 {
		p = 99
	}
	g.Progress = p
}

// handleListGoals отдаёт цели пользователя с прогрессом, вычисленным из задач.
func handleListGoals(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	list := goals.list(sessData.username)
	for i := range list {
		list[i].computeProgress()
	}
	c.JSON(http.StatusOK, gin.H{"goals": list})
}

// handleCreateGoal создаёт новую цель и, если переданы черновики задач
// (поле tasks, например из ИИ-генерации), сразу сохраняет их, привязав к цели.
func handleCreateGoal(c *gin.Context) {
	var req struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		TargetDate  string          `json:"target_date"`
		Status      string          `json:"status"`
		Tasks       []goalTaskDraft `json:"tasks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = goalActive
	}
	// Валидация черновиков до создания цели: заголовки задач не должны быть пустыми.
	// Отдельного предела на число задач нет — сколько пользователь оставил,
	// столько и создаётся.
	for _, d := range req.Tasks {
		if strings.TrimSpace(d.Title) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "У задачи цели пустой заголовок"})
			return
		}
	}

	sessData, _ := c.MustGet("session").(session)
	g, err := goals.create(sessData.username, req.Title, req.Description, req.TargetDate, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, d := range req.Tasks {
		title := strings.TrimSpace(d.Title)
		if title == "" {
			continue
		}
		category := strings.TrimSpace(d.Category)
		if category == "" {
			category = "Прочее"
		}
		hours := d.PlannedHours
		if hours < 0 {
			hours = 0
		}
		if _, err := tasks.create(sessData.username, category, title, d.Description,
			hours, 0, "", taskTodo, &g.ID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Не удалось создать задачу «" + title + "»: " + err.Error(),
			})
			return
		}
	}

	g.computeProgress()
	c.JSON(http.StatusOK, g)
}

// handleGenerateGoalTasks генерирует черновики задач для новой цели через
// DeepSeek. Ничего не сохраняет — только предлагает список, который показывается
// в форме создания цели до её сохранения.
func handleGenerateGoalTasks(c *gin.Context) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Сначала укажите название цели"})
		return
	}

	apiKey := deepseekAPIKey()
	if apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	drafts, err := aiGenerateGoalTasks(req.Title, req.Description, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": drafts})
}

// aiGenerateGoalTasks разбивает цель пользователя на конкретные задачи.
// Возвращает черновики задач; на БД они не сохраняются.
func aiGenerateGoalTasks(title, description, apiKey string) ([]goalTaskDraft, error) {
	allowed := strings.Join(TaskCategories, ", ")
	goalText := "Цель: " + title
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		goalText += "\nОписание: " + trimmed
	}

	systemPrompt := "Ты — планировщик, который разбивает долгосрочные цели на конкретные задачи. " +
		"По умолчанию разбей цель на 5–8 понятных последовательных задач (шагов). " +
		"Если в названии или описании цели пользователь явно указал количество задач — " +
		"сделай ровно столько, сколько он просит. " +
		"Верни СТРОГО валидный JSON без текста вне него, вида: " +
		"{\"tasks\": [{\"title\": string, \"description\": string, \"category\": string, \"planned_hours\": число}]}. " +
		"Правила: title — короткий заголовок-действие (до 60 символов, с глаголом); " +
		"description — одно-два предложения, что именно сделать (можно пустую строку); " +
		"category — ТОЛЬКО одно из значений: " + allowed + "; " +
		"planned_hours — реалистичная оценка в часах (число больше нуля, можно дробное). " +
		"Задачи должны в сумме приводить к достижению цели."

	payload := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": goalText},
		},
		"stream":          false,
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	hreq, err := http.NewRequest(http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("ошибка вызова DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek вернул статус %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("DeepSeek не вернул ответ")
	}

	return parseGoalTaskDrafts(parsed.Choices[0].Message.Content)
}

// parseGoalTaskDrafts разбирает JSON-ответ модели в черновики задач и
// нормализует поля (категория из разрешённого списка, часы >= 0).
func parseGoalTaskDrafts(content string) ([]goalTaskDraft, error) {
	// Модель может обернуть JSON в ```json ... ``` — вырезаем содержимое.
	s := strings.TrimSpace(content)
	if start := strings.Index(s, "{"); start > 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			s = s[start : end+1]
		}
	}

	var parsed struct {
		Tasks []struct {
			Title        string  `json:"title"`
			Description  string  `json:"description"`
			Category     string  `json:"category"`
			PlannedHours float64 `json:"planned_hours"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, fmt.Errorf("не удалось разобрать ответ модели: %w", err)
	}

	validCategory := make(map[string]bool, len(TaskCategories))
	for _, c := range TaskCategories {
		validCategory[c] = true
	}

	out := make([]goalTaskDraft, 0, len(parsed.Tasks))
	for _, t := range parsed.Tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		category := strings.TrimSpace(t.Category)
		if !validCategory[category] {
			category = "Прочее"
		}
		hours := t.PlannedHours
		if hours < 0 {
			hours = 0
		}
		out = append(out, goalTaskDraft{
			Title:        title,
			Description:  strings.TrimSpace(t.Description),
			Category:     category,
			PlannedHours: hours,
		})
		if len(out) >= 12 {
			break
		}
	}
	return out, nil
}

// handleUpdateGoal обновляет цель.
func handleUpdateGoal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID цели"})
		return
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		TargetDate  string `json:"target_date"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	g, err := goals.update(sessData.username, id, req.Title, req.Description, req.TargetDate, req.Status)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "цель не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	g.computeProgress()
	c.JSON(http.StatusOK, g)
}

// handleReorderGoalTasks задаёт последовательность задач цели.
// Тело: {"task_ids": [3, 1, 2]} — полный список задач цели в нужном порядке.
func handleReorderGoalTasks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID цели"})
		return
	}
	var req struct {
		TaskIDs []int `json:"task_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if len(req.TaskIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Список задач пуст"})
		return
	}

	sessData, _ := c.MustGet("session").(session)
	if _, ok := goals.getOwned(sessData.username, id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "цель не найдена"})
		return
	}
	if err := tasks.setGoalOrder(sessData.username, id, req.TaskIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleDeleteGoal удаляет цель. Параметр ?delete_tasks=1 удаляет также
// привязанные к цели задачи.
func handleDeleteGoal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID цели"})
		return
	}
	q := c.Query("delete_tasks")
	deleteTasks := q == "1" || strings.EqualFold(q, "true")

	sessData, _ := c.MustGet("session").(session)
	if err := goals.delete(sessData.username, id, deleteTasks); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
