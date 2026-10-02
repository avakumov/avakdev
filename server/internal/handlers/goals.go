package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// goalTaskDraft — черновик задачи, который пользователь получил от ИИ или
// добавил при создании цели. Сохраняется вместе с целью в момент создания.
type goalTaskDraft struct {
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Category     string  `json:"category"`
	PlannedHours float64 `json:"planned_hours"`
}

// normalizeGoalTaskDraft приводит черновик к допустимому виду: подрезает пробелы,
// категорию берёт только из предопределённых, часы не допускает отрицательными.
func normalizeGoalTaskDraft(d goalTaskDraft) goalTaskDraft {
	d.Title = strings.TrimSpace(d.Title)
	d.Description = strings.TrimSpace(d.Description)
	d.Category = strings.TrimSpace(d.Category)
	if !slices.Contains(app.TaskCategories, d.Category) {
		d.Category = "Прочее"
	}
	if d.PlannedHours < 0 {
		d.PlannedHours = 0
	}
	return d
}

// ListGoals отдаёт цели пользователя с прогрессом, вычисленным из задач.
func (h *Handlers) ListGoals(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	list := h.App.Goals.List(sessData.Username)
	for i := range list {
		h.App.Goals.ComputeProgress(&list[i])
	}
	c.JSON(http.StatusOK, httpkit.H{"goals": list})
}

// CreateGoal создаёт новую цель и, если переданы черновики задач
// (поле tasks, например из ИИ-генерации), сразу сохраняет их, привязав к цели.
func (h *Handlers) CreateGoal(c *httpkit.Context) {
	var req struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		TargetDate  string          `json:"target_date"`
		Status      string          `json:"status"`
		Tasks       []goalTaskDraft `json:"tasks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = app.GoalActive
	}
	// Валидация черновиков до создания цели: заголовки задач не должны быть пустыми.
	// Отдельного предела на число задач нет — сколько пользователь оставил,
	// столько и создаётся.
	for _, d := range req.Tasks {
		if strings.TrimSpace(d.Title) == "" {
			c.JSON(http.StatusBadRequest, httpkit.H{"error": "У задачи цели пустой заголовок"})
			return
		}
	}

	sessData, _ := c.MustGet("session").(app.Session)
	g, err := h.App.Goals.Create(sessData.Username, req.Title, req.Description, req.TargetDate, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}

	for _, draft := range req.Tasks {
		d := normalizeGoalTaskDraft(draft)
		if _, err := h.App.Tasks.Create(sessData.Username, d.Category, d.Title, d.Description,
			d.PlannedHours, "", app.TaskTodo, &g.ID); err != nil {
			c.JSON(http.StatusBadRequest, httpkit.H{
				"error": "Не удалось создать задачу «" + d.Title + "»: " + err.Error(),
			})
			return
		}
	}

	h.App.Goals.ComputeProgress(&g)
	c.JSON(http.StatusOK, g)
}

// GenerateGoalTasks генерирует черновики задач для новой цели через
// DeepSeek. Ничего не сохраняет — только предлагает список, который показывается
// в форме создания цели до её сохранения.
func (h *Handlers) GenerateGoalTasks(c *httpkit.Context) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Сначала укажите название цели"})
		return
	}

	apiKey := app.DeepSeekAPIKey()
	if apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, httpkit.H{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	drafts, truncated, err := aiGenerateGoalTasks(req.Title, req.Description, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"tasks": drafts, "truncated": truncated})
}

// taskCountRe — «28 задач», «на 12 шагов», «7 этапов» и т. п.
var taskCountRe = regexp.MustCompile(`(?i)(\d{1,3})\s*(задач|шаг|пункт|этап|част)`)

// requestedTaskCount ищет в тексте цели явно указанное число задач.
// 0 — количество не указано (тогда работает диапазон по умолчанию).
func requestedTaskCount(title, description string) int {
	for _, text := range []string{title, description} {
		m := taskCountRe.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 1 && n <= 999 {
			return n
		}
	}
	return 0
}

// aiGenerateGoalTasks разбивает цель пользователя на конкретные задачи.
// Возвращает черновики задач; на БД они не сохраняются. Признак truncated —
// ответ модели обрезан по лимиту вывода, то есть задач может не хватать.
func aiGenerateGoalTasks(title, description, apiKey string) (drafts []goalTaskDraft, truncated bool, err error) {
	allowed := strings.Join(app.TaskCategories, ", ")
	goalText := "Цель: " + title
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		goalText += "\nОписание: " + trimmed
	}

	// Если пользователь назвал число задач, просим его прямо в сообщении:
	// условные правила в длинном системном промпте модель выполняет хуже.
	want := requestedTaskCount(title, description)
	countRule := "По умолчанию разбей цель на 5–8 понятных последовательных задач (шагов). "
	if want > 0 {
		countRule = fmt.Sprintf("Пользователь просит ровно %d задач — верни ровно %d, ни больше ни меньше. ", want, want)
		goalText += fmt.Sprintf("\n\nНужно ровно %d задач.", want)
	}

	systemPrompt := "Ты — планировщик, который разбивает долгосрочные цели на конкретные задачи. " +
		countRule +
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
		"stream": false,
		// Явный лимит вывода: не полагаемся на значение по умолчанию, чтобы
		// большое число задач не обрезалось на середине.
		"max_tokens":      8192,
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}

	hreq, err := http.NewRequest(http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка вызова DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("DeepSeek вернул статус %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, false, err
	}
	if len(parsed.Choices) == 0 {
		return nil, false, fmt.Errorf("DeepSeek не вернул ответ")
	}

	choice := parsed.Choices[0]
	truncated = choice.FinishReason == "length"
	drafts, err = parseGoalTaskDrafts(choice.Message.Content)
	// Пишем в лог, сколько задач просили и сколько вернула модель: по этому логу
	// видно, если ответ обрезан или модель проигнорировала количество.
	log.Printf("GOALS: задач запрошено: %d, получено: %d, finish_reason=%s, tokens=%d",
		want, len(drafts), choice.FinishReason, parsed.Usage.CompletionTokens)
	if err != nil {
		if truncated {
			return nil, true, fmt.Errorf("ответ модели обрезан по лимиту длины — попробуйте ещё раз или уменьшите число задач")
		}
		return nil, false, err
	}
	return drafts, truncated, nil
}

// parseGoalTaskDrafts разбирает JSON-ответ модели в черновики задач
// (нормализацию делает normalizeGoalTaskDraft).
// Количество задач не ограничиваем: сколько просили и сколько вернула модель —
// столько и отдаём (раньше здесь молча отбрасывалось всё после 12-й задачи).
func parseGoalTaskDrafts(content string) ([]goalTaskDraft, error) {
	// Модель может обернуть JSON в ```json ... ``` — вырезаем содержимое.
	s := strings.TrimSpace(content)
	if start := strings.Index(s, "{"); start >= 0 {
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

	out := make([]goalTaskDraft, 0, len(parsed.Tasks))
	for _, t := range parsed.Tasks {
		d := normalizeGoalTaskDraft(goalTaskDraft{
			Title:        t.Title,
			Description:  t.Description,
			Category:     t.Category,
			PlannedHours: t.PlannedHours,
		})
		if d.Title == "" {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// UpdateGoal обновляет цель.
func (h *Handlers) UpdateGoal(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID цели"})
		return
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		TargetDate  string `json:"target_date"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	g, err := h.App.Goals.Update(sessData.Username, id, req.Title, req.Description, req.TargetDate, req.Status)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrGoalNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	h.App.Goals.ComputeProgress(&g)
	c.JSON(http.StatusOK, g)
}

// ReorderGoalTasks задаёт последовательность задач цели.
// Тело: {"task_ids": [3, 1, 2]} — полный список задач цели в нужном порядке.
func (h *Handlers) ReorderGoalTasks(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID цели"})
		return
	}
	var req struct {
		TaskIDs []int `json:"task_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	if len(req.TaskIDs) == 0 {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Список задач пуст"})
		return
	}

	sessData, _ := c.MustGet("session").(app.Session)
	if _, ok := h.App.Goals.GetOwned(sessData.Username, id); !ok {
		c.JSON(http.StatusNotFound, httpkit.H{"error": app.ErrGoalNotFound.Error()})
		return
	}
	if err := h.App.Tasks.SetGoalOrder(sessData.Username, id, req.TaskIDs); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}

// DeleteGoal удаляет цель. Параметр ?delete_tasks=1 удаляет также
// привязанные к цели задачи.
func (h *Handlers) DeleteGoal(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID цели"})
		return
	}
	q := c.Query("delete_tasks")
	deleteTasks := q == "1" || strings.EqualFold(q, "true")

	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Goals.Delete(sessData.Username, id, deleteTasks); err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
