package handlers

import (
	"net/http"
	"time"
)

// Health — health-check (только для администраторов).
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Message — демонстрационный ответ сервера.
func (h *Handlers) Message(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Привет! Это ответ от Go (net/http) сервера 🚀",
		"server":  "net/http",
	})
}

// ServerMetrics — системные метрики сервера (CPU, память, диск, сеть).
func (h *Handlers) ServerMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, CollectServerMetrics())
}
