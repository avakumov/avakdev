package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// ListTasks отдаёт задачи пользователя, категории и цели
// (для выбора/отображения привязки задачи к цели).
func (h *Handlers) ListTasks(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)

	// Лёгкое представление целей пользователя: только id, название, статус.
	allGoals := h.App.Goals.List(sessData.Username)
	brief := make([]struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}, 0, len(allGoals))
	for _, g := range allGoals {
		brief = append(brief, struct {
			ID     int    `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}{g.ID, g.Title, g.Status})
	}

	c.JSON(http.StatusOK, httpkit.H{
		"tasks":      h.App.Tasks.WithSpent(sessData.Username, h.App.Tasks.List(sessData.Username)),
		"categories": app.TaskCategories,
		"goals":      brief,
	})
}

// CreateTask создаёт новую задачу.
func (h *Handlers) CreateTask(c *httpkit.Context) {
	var req struct {
		Category      string  `json:"category"`
		Title         string  `json:"title"`
		Description   string  `json:"description"`
		PlannedHours  float64 `json:"planned_hours"`
		Deadline      string  `json:"deadline"`
		Status        string  `json:"status"`
		GoalID        *int    `json:"goal_id"`
		CompletedDate string  `json:"completed_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = app.TaskTodo
	}
	doneDate, ok := optionalDay(c, req.CompletedDate)
	if !ok {
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	t, err := h.App.Tasks.Create(sessData.Username, req.Category, req.Title, req.Description, req.PlannedHours, req.Deadline, req.Status, req.GoalID, doneDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, h.App.Tasks.WithSpent(sessData.Username, []app.Task{t})[0])
}

// optionalDay проверяет необязательную дату из запроса (ГГГГ-ММ-ДД).
// Пустая строка — допустима и означает «не указана». При ошибке форматирования
// отвечает 400 и возвращает ok=false.
func optionalDay(c *httpkit.Context, raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	day, err := app.ParseDay(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return "", false
	}
	return day, true
}

// UpdateTask обновляет задачу.
func (h *Handlers) UpdateTask(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID задачи"})
		return
	}
	var req struct {
		Category      string  `json:"category"`
		Title         string  `json:"title"`
		Description   string  `json:"description"`
		PlannedHours  float64 `json:"planned_hours"`
		Deadline      string  `json:"deadline"`
		Status        string  `json:"status"`
		GoalID        *int    `json:"goal_id"`
		CompletedDate string  `json:"completed_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	doneDate, ok := optionalDay(c, req.CompletedDate)
	if !ok {
		return
	}
	t, err := h.App.Tasks.Update(sessData.Username, id, req.Category, req.Title, req.Description, req.PlannedHours, req.Deadline, req.Status, req.GoalID, doneDate)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "задача не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, h.App.Tasks.WithSpent(sessData.Username, []app.Task{t})[0])
}

// DeleteTask удаляет задачу.
func (h *Handlers) DeleteTask(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Tasks.Delete(sessData.Username, id); err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
