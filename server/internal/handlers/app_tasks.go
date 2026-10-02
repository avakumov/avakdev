package handlers

import (
	"net/http"
	"strconv"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// ListAppTasks отдаёт задачи пользователя.
func (h *Handlers) ListAppTasks(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	c.JSON(http.StatusOK, httpkit.H{"tasks": h.App.AppTasks.List(sessData.Username)})
}

// CreateAppTask создаёт новую задачу.
func (h *Handlers) CreateAppTask(c *httpkit.Context) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	t, err := h.App.AppTasks.Create(sessData.Username, req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// UpdateAppTask обновляет задачу (заголовок, описание, статус).
func (h *Handlers) UpdateAppTask(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID задачи"})
		return
	}
	var req struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Status          string  `json:"status"`
		Result          *string `json:"result"`
		Log             *string `json:"log"`
		DeployRequested *bool   `json:"deploy_requested"`
		DeployedAt      *string `json:"deployed_at"`
		CommitHash      *string `json:"commit_hash"`
		RevertRequested *bool   `json:"revert_requested"`
		RevertedAt      *string `json:"reverted_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	t, err := h.App.AppTasks.Update(sessData.Username, id, req.Title, req.Description, req.Status, req.Result, req.Log, req.DeployRequested, req.DeployedAt, req.CommitHash, req.RevertRequested, req.RevertedAt)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "задача не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// DeleteAppTask удаляет задачу.
func (h *Handlers) DeleteAppTask(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.AppTasks.Delete(sessData.Username, id); err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
