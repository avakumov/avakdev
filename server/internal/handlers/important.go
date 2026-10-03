package handlers

import (
	"net/http"

	"avakumov/server/internal/app"
)

// GetImportant отдаёт «важное» сообщение текущего пользователя:
// текст, автора и время последнего обновления, а также флаг enabled
// (показ только на production) и seen_today (показывается раз в сутки).
func (h *Handlers) GetImportant(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	msg, _ := h.App.Important.Get(sessData.Username)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    app.ImportantEnabled(),
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
		"seen_today": h.App.Important.SeenToday(sessData.Username),
	})
}

// SaveImportant сохраняет «важное» сообщение текущего пользователя.
// Каждый авторизованный пользователь управляет только своим сообщением.
func (h *Handlers) SaveImportant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	msg, err := h.App.Important.Save(sessData.Username, req.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить сообщение"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
	})
}

// MarkImportantSeen отмечает, что текущий пользователь прочитал
// сообщение сегодня — до следующего дня оно ему больше не покажется.
func (h *Handlers) MarkImportantSeen(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	if err := h.App.Important.MarkSeen(sessData.Username); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось отметить сообщение прочитанным"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
