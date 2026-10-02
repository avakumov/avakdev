// Package handlers — HTTP-слой: обработчики маршрутов. Зависимости (хранилища,
// сессии) передаются в Handlers, поэтому логика данных живёт в internal/store и
// internal/app, а не в глобальном состоянии пакета.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// Handlers — HTTP-обработчики приложения.
type Handlers struct {
	App *app.App
}

// New создаёт набор обработчиков на готовом приложении.
func New(a *app.App) *Handlers { return &Handlers{App: a} }

// maxDraftRunes — предельный размер текста заметки (символов).
const maxDraftRunes = 20000

// ListDrafts возвращает заметки пользователя: свежие сверху.
func (h *Handlers) ListDrafts(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	out, err := h.App.Drafts.List(context.Background(), sessData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить заметки"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// CreateDraft сохраняет новую заметку.
func (h *Handlers) CreateDraft(c *httpkit.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)

	n, err := h.App.Drafts.Create(context.Background(), sessData.Username, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить заметку"})
		return
	}
	c.JSON(http.StatusOK, n)
}

// draftContent проверяет и нормализует текст заметки.
func draftContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	if content == "" {
		return "", errors.New("заметка пустая")
	}
	if len([]rune(content)) > maxDraftRunes {
		return "", errors.New("заметка слишком длинная")
	}
	return content, nil
}

// UpdateDraft заменяет текст существующей заметки.
func (h *Handlers) UpdateDraft(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID заметки"})
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)

	n, err := h.App.Drafts.Update(context.Background(), sessData.Username, id, content)
	if err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, n)
}

// DeleteDraft удаляет заметку.
func (h *Handlers) DeleteDraft(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID заметки"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	ok, err := h.App.Drafts.Delete(context.Background(), sessData.Username, id)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
