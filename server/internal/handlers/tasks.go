package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
)

// ListTasks отдаёт задачи пользователя, категории и цели
// (для выбора/отображения привязки задачи к цели).
func (h *Handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)

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

	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":      h.App.Tasks.WithSpent(sessData.Username, h.App.Tasks.List(sessData.Username)),
		"categories": app.TaskCategories,
		"goals":      brief,
	})
}

// CreateTask создаёт новую задачу.
func (h *Handlers) CreateTask(w http.ResponseWriter, r *http.Request) {
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
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = app.TaskTodo
	}
	doneDate, ok := optionalDay(w, req.CompletedDate)
	if !ok {
		return
	}
	sessData, _ := sessionOf(r)
	t, err := h.App.Tasks.Create(sessData.Username, req.Category, req.Title, req.Description, req.PlannedHours, req.Deadline, req.Status, req.GoalID, doneDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.App.Tasks.WithSpent(sessData.Username, []app.Task{t})[0])
}

// optionalDay проверяет необязательную дату из запроса (ГГГГ-ММ-ДД).
// Пустая строка — допустима и означает «не указана». При ошибке форматирования
// отвечает 400 и возвращает ok=false.
func optionalDay(w http.ResponseWriter, raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	day, err := app.ParseDay(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return "", false
	}
	return day, true
}

// UpdateTask обновляет задачу.
func (h *Handlers) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID задачи"})
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
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	doneDate, ok := optionalDay(w, req.CompletedDate)
	if !ok {
		return
	}
	t, err := h.App.Tasks.Update(sessData.Username, id, req.Category, req.Title, req.Description, req.PlannedHours, req.Deadline, req.Status, req.GoalID, doneDate)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "задача не найдена" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.App.Tasks.WithSpent(sessData.Username, []app.Task{t})[0])
}

// DeleteTask удаляет задачу.
func (h *Handlers) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.Tasks.Delete(sessData.Username, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
