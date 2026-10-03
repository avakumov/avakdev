package handlers

import (
	"context"
	"net/http"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/store"
)

// ListReports возвращает текстовые отчёты пользователя (только дни с непустым
// текстом), новые сверху.
func (h *Handlers) ListReports(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	out, err := h.App.Reports.List(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить отчёты"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// UpsertReport создаёт или обновляет отчёт за конкретный день (PUT /api/reports/:date).
func (h *Handlers) UpsertReport(w http.ResponseWriter, r *http.Request) {
	day, err := app.ParseDay(param(r, "date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}

	sessData, _ := sessionOf(r)
	if err := h.App.Reports.Upsert(context.Background(), sessData.Username, day, req.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить отчёт"})
		return
	}

	writeJSON(w, http.StatusOK, store.Report{
		Date:    day,
		Content: req.Content,
		Updated: time.Now().UTC().Format(time.RFC3339),
	})
}
