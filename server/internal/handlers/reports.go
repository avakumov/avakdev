package handlers

import (
	"context"
	"net/http"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
	"avakumov/server/internal/store"
)

// ListReports возвращает текстовые отчёты пользователя (только дни с непустым
// текстом), новые сверху.
func (h *Handlers) ListReports(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	out, err := h.App.Reports.List(context.Background(), sessData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить отчёты"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// UpsertReport создаёт или обновляет отчёт за конкретный день (PUT /api/reports/:date).
func (h *Handlers) UpsertReport(c *httpkit.Context) {
	day, err := app.ParseDay(c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}

	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Reports.Upsert(context.Background(), sessData.Username, day, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить отчёт"})
		return
	}

	c.JSON(http.StatusOK, store.Report{
		Date:    day,
		Content: req.Content,
		Updated: time.Now().UTC().Format(time.RFC3339),
	})
}
