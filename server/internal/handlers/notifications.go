package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// ListNotifications отдаёт уведомления пользователя.
func (h *Handlers) ListNotifications(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	c.JSON(http.StatusOK, httpkit.H{"notifications": h.App.Notifications.List(sessData.Username)})
}

// CreateNotification создаёт уведомление.
func (h *Handlers) CreateNotification(c *httpkit.Context) {
	var req struct {
		Text        string `json:"text"`
		Type        string `json:"type"`
		DueAt       string `json:"due_at"`
		PeriodUnit  string `json:"period_unit"`
		PeriodValue int    `json:"period_value"`
		Channel     string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
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
	sessData, _ := c.MustGet("session").(app.Session)
	n, err := h.App.Notifications.Create(sessData.Username, req.Text, req.Type, req.DueAt, req.PeriodUnit, req.PeriodValue, req.Channel)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

// DeleteNotification удаляет уведомление.
func (h *Handlers) DeleteNotification(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Notifications.Delete(sessData.Username, id); err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
