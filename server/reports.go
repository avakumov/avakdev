package main

import (
	"context"
	"net/http"
	"time"

	"avakumov/server/internal/httpkit"
	"avakumov/server/internal/store"
)

// Текстовые отчёты за день. Хранятся в day_plans.report (одна строка на
// пользователя и дату). SQL живёт в store.Reports (internal/store/reports.go).

// handleListReports возвращает текстовые отчёты текущего пользователя
// (только даты с непустым текстом), новые сверху.
func handleListReports(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(session)
	out, err := reportsStore.List(context.Background(), sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить отчёты"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleUpsertReport создаёт или обновляет текстовый отчёт за конкретный день.
// Отчёт хранится в строке дня (day_plans): если дня ещё нет — строка
// создаётся с пустым планом. Параметр :date — день в формате YYYY-MM-DD.
func handleUpsertReport(c *httpkit.Context) {
	day, err := parseDay(c.Param("date"))
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

	sessData, _ := c.MustGet("session").(session)
	if err := reportsStore.Upsert(context.Background(), sessData.username, day, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить отчёт"})
		return
	}

	c.JSON(http.StatusOK, store.Report{
		Date:    day,
		Content: req.Content,
		Updated: time.Now().UTC().Format(time.RFC3339),
	})
}
