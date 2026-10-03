package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// maxDraftRunes — предельный размер текста заметки (символов).
const maxDraftRunes = 20000

// ListDrafts возвращает заметки пользователя: свежие сверху.
func (h *Handlers) ListDrafts(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	out, err := h.App.Drafts.List(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить заметки"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateDraft сохраняет новую заметку.
func (h *Handlers) CreateDraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)

	n, err := h.App.Drafts.Create(context.Background(), sessData.Username, content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить заметку"})
		return
	}
	writeJSON(w, http.StatusOK, n)
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
func (h *Handlers) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID заметки"})
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)

	n, err := h.App.Drafts.Update(context.Background(), sessData.Username, id, content)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Заметка не найдена"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// DeleteDraft удаляет заметку.
func (h *Handlers) DeleteDraft(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID заметки"})
		return
	}
	sessData, _ := sessionOf(r)
	ok, err := h.App.Drafts.Delete(context.Background(), sessData.Username, id)
	if err != nil || !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Заметка не найдена"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
