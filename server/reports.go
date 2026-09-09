package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Report — текстовый отчёт за день пользователя.
// Хранится в day_plans.report (одна строка на пользователя и дату),
// поэтому отдельной таблицы reports больше нет.
type Report struct {
	// Date — день отчёта в формате YYYY-MM-DD.
	Date string `json:"date"`
	// Content — текст отчёта.
	Content string `json:"content"`
	// Updated — время последнего изменения (RFC3339, UTC).
	Updated string `json:"updated"`
}

// handleListReports возвращает текстовые отчёты текущего пользователя
// (только даты с непустым текстом), новые сверху.
func handleListReports(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	rows, err := db.Query(context.Background(),
		`SELECT to_char(day,'YYYY-MM-DD'),
		        report,
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM day_plans
		 WHERE username = $1 AND report <> ''
		 ORDER BY day DESC`,
		sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить отчёты"})
		return
	}
	defer rows.Close()

	out := make([]Report, 0)
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.Date, &r.Content, &r.Updated); err == nil {
			out = append(out, r)
		}
	}
	c.JSON(http.StatusOK, out)
}

// handleUpsertReport создаёт или обновляет текстовый отчёт за конкретный день.
// Отчёт хранится в строке дня (day_plans): если дня ещё нет — строка
// создаётся с пустым планом. Параметр :date — день в формате YYYY-MM-DD.
func handleUpsertReport(c *gin.Context) {
	day, err := parseDay(c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	sessData, _ := c.MustGet("session").(session)
	ctx := context.Background()

	// Гарантируем наличие строки дня (например, отчёт без сохранённого плана).
	if _, err := db.Exec(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, 0)
		 ON CONFLICT (username, day) DO NOTHING`,
		sessData.username, day); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить отчёт"})
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE day_plans SET report = $3, updated = now()
		 WHERE username = $1 AND day = $2`,
		sessData.username, day, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить отчёт"})
		return
	}

	c.JSON(http.StatusOK, Report{
		Date:    day,
		Content: req.Content,
		Updated: time.Now().UTC().Format(time.RFC3339),
	})
}
