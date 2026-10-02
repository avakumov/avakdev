package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"avakumov/server/internal/httpkit"
)

// Раздел «Заметки»: быстрые записи-черновики. В коде называются draft, чтобы не
// путать с конспектами раздела «Знания» (knowledge_notes). Текст один — либо
// пишем с нуля в плавающем окне, либо правим уже сохранённую запись.
// SQL живёт в store.Drafts (см. internal/store/drafts.go).

// maxDraftRunes — предельный размер текста заметки (символов).
const maxDraftRunes = 20000

// handleListDrafts возвращает заметки пользователя: свежие сверху.
func handleListDrafts(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(session)
	out, err := draftsStore.List(context.Background(), sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить заметки"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleCreateDraft сохраняет новую заметку.
func handleCreateDraft(c *httpkit.Context) {
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
	sessData, _ := c.MustGet("session").(session)

	n, err := draftsStore.Create(context.Background(), sessData.username, content)
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

// handleUpdateDraft заменяет текст существующей заметки.
func handleUpdateDraft(c *httpkit.Context) {
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
	sessData, _ := c.MustGet("session").(session)

	n, err := draftsStore.Update(context.Background(), sessData.username, id, content)
	if err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleDeleteDraft удаляет заметку.
func handleDeleteDraft(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID заметки"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	ok, err := draftsStore.Delete(context.Background(), sessData.username, id)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
