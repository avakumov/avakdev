package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// maxBookmarkExcerpt — сколько символов фрагмента храним (для списка закладок).
const maxBookmarkExcerpt = 300

// LastBookmark возвращает последнюю добавленную закладку пользователя (по всем
// книгам). Прочитанные книги не берём. Если закладок нет — отдаёт null.
func (h *Handlers) LastBookmark(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	out, err := h.App.Bookmarks.Last(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить последнюю закладку"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ListBookmarks возвращает закладки книги в порядке по тексту.
func (h *Handlers) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := sessionOf(r)
	if !h.App.Bookmarks.BookOwned(context.Background(), sessData.Username, bookID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Книга не найдена"})
		return
	}

	out, err := h.App.Bookmarks.List(context.Background(), sessData.Username, bookID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить закладки"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateBookmark сохраняет закладку на выделенном фрагменте.
func (h *Handlers) CreateBookmark(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := sessionOf(r)
	if !h.App.Bookmarks.BookOwned(context.Background(), sessData.Username, bookID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Книга не найдена"})
		return
	}

	var req struct {
		Anchor  *int   `json:"anchor"`
		Excerpt string `json:"excerpt"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Anchor == nil || *req.Anchor < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректная позиция закладки"})
		return
	}
	excerpt := req.Excerpt
	if strings.TrimSpace(excerpt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Выделите текст для закладки"})
		return
	}
	if runes := []rune(excerpt); len(runes) > maxBookmarkExcerpt {
		excerpt = string(runes[:maxBookmarkExcerpt])
	}

	b, err := h.App.Bookmarks.Create(context.Background(), sessData.Username, bookID, *req.Anchor, excerpt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить закладку"})
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// DeleteBookmark удаляет закладку книги.
func (h *Handlers) DeleteBookmark(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID книги"})
		return
	}
	bookmarkID, err := strconv.Atoi(param(r, "bookmarkId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID закладки"})
		return
	}
	sessData, _ := sessionOf(r)
	ok, err := h.App.Bookmarks.Delete(context.Background(), sessData.Username, bookID, bookmarkID)
	if err != nil || !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Закладка не найдена"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
