package handlers

import (
	"net/http"
	"strconv"
)

// ListAppTasks отдаёт задачи пользователя.
func (h *Handlers) ListAppTasks(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	writeJSON(w, http.StatusOK, map[string]any{"tasks": h.App.AppTasks.List(sessData.Username)})
}

// CreateAppTask создаёт новую задачу.
func (h *Handlers) CreateAppTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	t, err := h.App.AppTasks.Create(sessData.Username, req.Title, req.Description)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// UpdateAppTask обновляет задачу (заголовок, описание, статус).
func (h *Handlers) UpdateAppTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID задачи"})
		return
	}
	var req struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Status          string  `json:"status"`
		Result          *string `json:"result"`
		Log             *string `json:"log"`
		DeployRequested *bool   `json:"deploy_requested"`
		DeployedAt      *string `json:"deployed_at"`
		CommitHash      *string `json:"commit_hash"`
		RevertRequested *bool   `json:"revert_requested"`
		RevertedAt      *string `json:"reverted_at"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	t, err := h.App.AppTasks.Update(sessData.Username, id, req.Title, req.Description, req.Status, req.Result, req.Log, req.DeployRequested, req.DeployedAt, req.CommitHash, req.RevertRequested, req.RevertedAt)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "задача не найдена" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// DeleteAppTask удаляет задачу.
func (h *Handlers) DeleteAppTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.AppTasks.Delete(sessData.Username, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
