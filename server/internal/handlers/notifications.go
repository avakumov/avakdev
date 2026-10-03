package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
)

// ListNotifications отдаёт уведомления пользователя.
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	writeJSON(w, http.StatusOK, map[string]any{"notifications": h.App.Notifications.List(sessData.Username)})
}

// CreateNotification создаёт уведомление.
func (h *Handlers) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text        string `json:"text"`
		Type        string `json:"type"`
		DueAt       string `json:"due_at"`
		PeriodUnit  string `json:"period_unit"`
		PeriodValue int    `json:"period_value"`
		Channel     string `json:"channel"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if req.Type == "" {
		req.Type = app.NotifOnce
	}
	if req.Channel == "" {
		req.Channel = app.NotifChannelApp
	}
	if req.PeriodValue == 0 {
		req.PeriodValue = 1
	}
	sessData, _ := sessionOf(r)
	n, err := h.App.Notifications.Create(sessData.Username, req.Text, req.Type, req.DueAt, req.PeriodUnit, req.PeriodValue, req.Channel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// DeleteNotification удаляет уведомление.
func (h *Handlers) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.Notifications.Delete(sessData.Username, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
