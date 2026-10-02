package handlers

import (
	"net/http"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// GetImportant отдаёт «важное» сообщение текущего пользователя:
// текст, автора и время последнего обновления, а также флаг enabled
// (показ только на production) и seen_today (показывается раз в сутки).
func (h *Handlers) GetImportant(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	msg, _ := h.App.Important.Get(sessData.Username)
	c.JSON(http.StatusOK, httpkit.H{
		"enabled":    app.ImportantEnabled(),
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
		"seen_today": h.App.Important.SeenToday(sessData.Username),
	})
}

// SaveImportant сохраняет «важное» сообщение текущего пользователя.
// Каждый авторизованный пользователь управляет только своим сообщением.
func (h *Handlers) SaveImportant(c *httpkit.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	msg, err := h.App.Important.Save(sessData.Username, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить сообщение"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
	})
}

// MarkImportantSeen отмечает, что текущий пользователь прочитал
// сообщение сегодня — до следующего дня оно ему больше не покажется.
func (h *Handlers) MarkImportantSeen(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Important.MarkSeen(sessData.Username); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось отметить сообщение прочитанным"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
