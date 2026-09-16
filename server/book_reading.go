package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Раздел «Чтение»: учёт времени чтения. Клиент считает секунды в модалке книги
// (отсчёт идёт, пока пользователь листает текст) и по закрытию книги добавляет
// их к дню. Дата — календарный день клиента в формате ГГГГ-ММ-ДД.
// Цель чтения у каждого дня своя (по умолчанию — час), её можно менять.

// readingGoalSeconds — цель чтения по умолчанию: час в день.
const readingGoalSeconds = 3600

// Рамки дневной цели чтения (1 минута … сутки).
const (
	minReadingGoalSeconds = 60
	maxReadingGoalSeconds = 24 * 3600
)

// maxReadingSecondsPerSave — ограничение на одно сохранение (сутки), чтобы
// случайный сбой клиента не записал в день мусорное значение.
const maxReadingSecondsPerSave = 24 * 3600

// readingTimeBody — ответ по времени чтения за день.
type readingTimeBody struct {
	Date        string `json:"date"`
	Seconds     int    `json:"seconds"`
	GoalSeconds int    `json:"goal_seconds"`
}

// handleGetReadingTime возвращает, сколько секунд пользователь читал за день
// и какова цель этого дня (GET /api/reading/time?date=ГГГГ-ММ-ДД; пусто — сегодня).
func handleGetReadingTime(c *gin.Context) {
	day, err := parseDay(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	seconds := 0
	goal := readingGoalSeconds
	err = db.QueryRow(context.Background(),
		`SELECT seconds, goal_seconds FROM book_reading WHERE username = $1 AND date = $2`,
		sessData.username, day).Scan(&seconds, &goal)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить время чтения"})
		return
	}
	c.JSON(http.StatusOK, readingTimeBody{
		Date:        day,
		Seconds:     seconds,
		GoalSeconds: goal,
	})
}

// handleAddReadingTime добавляет секунды чтения к дню (POST /api/reading/time,
// тело {"date","seconds"}) и возвращает новую сумму за день.
func handleAddReadingTime(c *gin.Context) {
	var req struct {
		Date    string `json:"date"`
		Seconds int    `json:"seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Seconds <= 0 || req.Seconds > maxReadingSecondsPerSave {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректное время чтения"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var total, goal int
	err = db.QueryRow(context.Background(),
		`INSERT INTO book_reading (username, date, seconds)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (username, date)
		 DO UPDATE SET seconds = book_reading.seconds + EXCLUDED.seconds
		 RETURNING seconds, goal_seconds`,
		sessData.username, day, req.Seconds).Scan(&total, &goal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить время чтения"})
		return
	}
	c.JSON(http.StatusOK, readingTimeBody{
		Date:        day,
		Seconds:     total,
		GoalSeconds: goal,
	})
}

// handleSetReadingGoal меняет цель чтения на день (PUT /api/reading/goal,
// тело {"date","goal_seconds"}). Строка дня создаётся, если её ещё нет.
func handleSetReadingGoal(c *gin.Context) {
	var req struct {
		Date        string `json:"date"`
		GoalSeconds int    `json:"goal_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.GoalSeconds < minReadingGoalSeconds || req.GoalSeconds > maxReadingGoalSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Цель чтения — от 1 минуты до 24 часов"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var seconds, goal int
	err = db.QueryRow(context.Background(),
		`INSERT INTO book_reading (username, date, seconds, goal_seconds)
		 VALUES ($1, $2, 0, $3)
		 ON CONFLICT (username, date)
		 DO UPDATE SET goal_seconds = EXCLUDED.goal_seconds
		 RETURNING seconds, goal_seconds`,
		sessData.username, day, req.GoalSeconds).Scan(&seconds, &goal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить цель чтения"})
		return
	}
	c.JSON(http.StatusOK, readingTimeBody{
		Date:        day,
		Seconds:     seconds,
		GoalSeconds: goal,
	})
}
