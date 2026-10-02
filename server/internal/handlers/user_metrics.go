package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// ListUserMetrics отдаёт определения метрик пользователя и все их значения.
func (h *Handlers) ListUserMetrics(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	defs := h.App.UserMetrics.List(sessData.Username)

	values := make([]app.MetricValue, 0)
	for _, d := range defs {
		for date, value := range h.App.UserMetrics.Values(d.ID) {
			values = append(values, app.MetricValue{MetricID: d.ID, Date: date, Value: value})
		}
	}
	c.JSON(http.StatusOK, httpkit.H{"definitions": defs, "values": values})
}

// CreateUserMetric создаёт новую метрику пользователя.
func (h *Handlers) CreateUserMetric(c *httpkit.Context) {
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Unit string `json:"unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	d, err := h.App.UserMetrics.Create(sessData.Username, req.Name, req.Type, req.Unit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

// UpdateUserMetric переименовывает метрику и меняет её единицу измерения.
func (h *Handlers) UpdateUserMetric(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID метрики"})
		return
	}
	var req struct {
		Name string `json:"name"`
		Unit string `json:"unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	d, err := h.App.UserMetrics.Update(sessData.Username, id, req.Name, req.Unit)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

// DeleteUserMetric удаляет метрику пользователя (со значениями).
func (h *Handlers) DeleteUserMetric(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID метрики"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.UserMetrics.Delete(sessData.Username, id); err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}

// SetUserMetricValue сохраняет показатель метрики за день.
func (h *Handlers) SetUserMetricValue(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID метрики"})
		return
	}
	date := c.Param("date")

	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.UserMetrics.SetValue(sessData.Username, id, date, req.Value); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"metric_id": id, "date": date, "value": req.Value})
}

// DeleteUserMetricValue удаляет показатель метрики за день.
func (h *Handlers) DeleteUserMetricValue(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID метрики"})
		return
	}
	date := c.Param("date")
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.UserMetrics.DeleteValue(sessData.Username, id, date); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
