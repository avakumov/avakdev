package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
)

// ListUserMetrics отдаёт определения метрик пользователя и все их значения.
func (h *Handlers) ListUserMetrics(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	defs := h.App.UserMetrics.List(sessData.Username)

	values := make([]app.MetricValue, 0)
	for _, d := range defs {
		for date, value := range h.App.UserMetrics.Values(d.ID) {
			values = append(values, app.MetricValue{MetricID: d.ID, Date: date, Value: value})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"definitions": defs, "values": values})
}

// CreateUserMetric создаёт новую метрику пользователя.
func (h *Handlers) CreateUserMetric(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Unit string `json:"unit"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	d, err := h.App.UserMetrics.Create(sessData.Username, req.Name, req.Type, req.Unit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// UpdateUserMetric переименовывает метрику и меняет её единицу измерения.
func (h *Handlers) UpdateUserMetric(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID метрики"})
		return
	}
	var req struct {
		Name string `json:"name"`
		Unit string `json:"unit"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	d, err := h.App.UserMetrics.Update(sessData.Username, id, req.Name, req.Unit)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// DeleteUserMetric удаляет метрику пользователя (со значениями).
func (h *Handlers) DeleteUserMetric(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID метрики"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.UserMetrics.Delete(sessData.Username, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SetUserMetricValue сохраняет показатель метрики за день.
func (h *Handlers) SetUserMetricValue(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID метрики"})
		return
	}
	date := param(r, "date")

	var req struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := sessionOf(r)
	if err := h.App.UserMetrics.SetValue(sessData.Username, id, date, req.Value); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"metric_id": id, "date": date, "value": req.Value})
}

// DeleteUserMetricValue удаляет показатель метрики за день.
func (h *Handlers) DeleteUserMetricValue(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID метрики"})
		return
	}
	date := param(r, "date")
	sessData, _ := sessionOf(r)
	if err := h.App.UserMetrics.DeleteValue(sessData.Username, id, date); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
