package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// maxHighlightExcerpt — сколько символов фрагмента выделения храним.
const maxHighlightExcerpt = 300

// highlightColors — допустимые id цветов выделения (совпадают с палитрой на
// клиенте). Держим список на сервере, чтобы принимать только известные значения.
var highlightColors = map[string]bool{
	"yellow": true,
	"green":  true,
	"blue":   true,
	"pink":   true,
}

// ListHighlights возвращает выделения книги в порядке по тексту.
func (h *Handlers) ListHighlights(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := sessionOf(r)
	if !h.App.Highlights.BookOwned(context.Background(), sessData.Username, bookID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Книга не найдена"})
		return
	}

	out, err := h.App.Highlights.List(context.Background(), sessData.Username, bookID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить выделения"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateHighlight сохраняет выделение на выбранном диапазоне текста.
func (h *Handlers) CreateHighlight(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := sessionOf(r)
	if !h.App.Highlights.BookOwned(context.Background(), sessData.Username, bookID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Книга не найдена"})
		return
	}

	var req struct {
		Start   *int   `json:"start"`
		End     *int   `json:"end"`
		Color   string `json:"color"`
		Excerpt string `json:"excerpt"`
	}
	if err := decodeJSON(r, &req); err != nil ||
		req.Start == nil || req.End == nil ||
		*req.Start < 0 || *req.End <= *req.Start {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный диапазон выделения"})
		return
	}
	if !highlightColors[strings.TrimSpace(req.Color)] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный цвет выделения"})
		return
	}
	excerpt := req.Excerpt
	if runes := []rune(excerpt); len(runes) > maxHighlightExcerpt {
		excerpt = string(runes[:maxHighlightExcerpt])
	}

	hl, err := h.App.Highlights.Create(context.Background(), sessData.Username, bookID, *req.Start, *req.End, req.Color, excerpt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить выделение"})
		return
	}
	writeJSON(w, http.StatusOK, hl)
}

// DeleteHighlight удаляет выделение книги.
func (h *Handlers) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	highlightID, err := strconv.Atoi(param(r, "highlightId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID выделения"})
		return
	}
	sessData, _ := sessionOf(r)
	ok, err := h.App.Highlights.Delete(context.Background(), sessData.Username, bookID, highlightID)
	if err != nil || !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Выделение не найдено"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
