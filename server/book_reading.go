package main

import (
	"net/http"

	"avakumov/server/internal/httpkit"
)

// Раздел «Чтение»: учёт времени чтения. Клиент считает секунды в модалке книги
// (отсчёт идёт, пока пользователь листает текст) и по закрытию книги добавляет
// их к дню. Дата — календарный день клиента в формате ГГГГ-ММ-ДД.
// Цель чтения у каждого дня своя (по умолчанию — час), её можно менять.
// Данные и SQL живут в store.Reading (переменная readingStore).

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

// readingHistoryDays — сколько последних дней с чтением отдаём отчётам.
const readingHistoryDays = 90

// handleReadingHistory — дни, в которые было чтение (сначала новые).
// Нужна отчётам: такие дни попадают в список дней наравне с планами.
// Дни с нулём секунд (например, только изменённая цель) не отдаём.
func handleReadingHistory(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(session)
	days, err := readingStore.History(c.Request.Context(), sessData.Username, readingHistoryDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить историю чтения"})
		return
	}
	c.JSON(http.StatusOK, days)
}

// handleGetReadingTime возвращает, сколько секунд пользователь читал за день
// и какова цель этого дня (GET /api/reading/time?date=ГГГГ-ММ-ДД; пусто — сегодня).
func handleGetReadingTime(c *httpkit.Context) {
	day, err := parseDay(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	res, err := readingStore.Day(c.Request.Context(), sessData.Username, day, readingGoalSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить время чтения"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// handleAddReadingTime добавляет секунды чтения к дню (POST /api/reading/time,
// тело {"date","seconds"}) и возвращает новую сумму за день.
func handleAddReadingTime(c *httpkit.Context) {
	var req struct {
		Date    string `json:"date"`
		Seconds int    `json:"seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	if req.Seconds <= 0 || req.Seconds > maxReadingSecondsPerSave {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректное время чтения"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	res, err := readingStore.Add(c.Request.Context(), sessData.Username, day, req.Seconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить время чтения"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// handleSetReadingGoal меняет цель чтения на день (PUT /api/reading/goal,
// тело {"date","goal_seconds"}). Строка дня создаётся, если её ещё нет.
func handleSetReadingGoal(c *httpkit.Context) {
	var req struct {
		Date        string `json:"date"`
		GoalSeconds int    `json:"goal_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	day, err := parseDay(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	if req.GoalSeconds < minReadingGoalSeconds || req.GoalSeconds > maxReadingGoalSeconds {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Цель чтения — от 1 минуты до 24 часов"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	res, err := readingStore.SetGoal(c.Request.Context(), sessData.Username, day, req.GoalSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить цель чтения"})
		return
	}
	c.JSON(http.StatusOK, res)
}
