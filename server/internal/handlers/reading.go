package handlers

import (
	"net/http"

	"avakumov/server/internal/app"
)

// Раздел «Чтение»: учёт времени чтения. Клиент считает секунды в модалке книги
// (отсчёт идёт, пока пользователь листает текст) и по закрытию книги добавляет
// их к дню. Дата — календарный день клиента в формате ГГГГ-ММ-ДД.
// Цель чтения у каждого дня своя (по умолчанию — час), её можно менять.
// Данные и SQL живут в store.Reading (App.Reading).

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

// ReadingHistory — дни, в которые было чтение (сначала новые).
// Нужна отчётам: такие дни попадают в список дней наравне с планами.
// Дни с нулём секунд (например, только изменённая цель) не отдаём.
func (h *Handlers) ReadingHistory(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	days, err := h.App.Reading.History(r.Context(), sessData.Username, readingHistoryDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить историю чтения"})
		return
	}
	writeJSON(w, http.StatusOK, days)
}

// GetReadingTime возвращает, сколько секунд пользователь читал за день
// и какова цель этого дня (GET /api/reading/time?date=ГГГГ-ММ-ДД; пусто — сегодня).
func (h *Handlers) GetReadingTime(w http.ResponseWriter, r *http.Request) {
	day, err := app.ParseDay(r.URL.Query().Get("date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)

	res, err := h.App.Reading.Day(r.Context(), sessData.Username, day, readingGoalSeconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить время чтения"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// AddReadingTime добавляет секунды чтения к дню (POST /api/reading/time,
// тело {"date","seconds"}) и возвращает новую сумму за день.
func (h *Handlers) AddReadingTime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date    string `json:"date"`
		Seconds int    `json:"seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Seconds <= 0 || req.Seconds > maxReadingSecondsPerSave {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректное время чтения"})
		return
	}
	sessData, _ := sessionOf(r)

	res, err := h.App.Reading.Add(r.Context(), sessData.Username, day, req.Seconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить время чтения"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// SetReadingGoal меняет цель чтения на день (PUT /api/reading/goal,
// тело {"date","goal_seconds"}). Строка дня создаётся, если её ещё нет.
func (h *Handlers) SetReadingGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date        string `json:"date"`
		GoalSeconds int    `json:"goal_seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	day, err := app.ParseDay(req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.GoalSeconds < minReadingGoalSeconds || req.GoalSeconds > maxReadingGoalSeconds {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Цель чтения — от 1 минуты до 24 часов"})
		return
	}
	sessData, _ := sessionOf(r)

	res, err := h.App.Reading.SetGoal(r.Context(), sessData.Username, day, req.GoalSeconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить цель чтения"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
