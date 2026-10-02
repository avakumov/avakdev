package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// maxBookmarkExcerpt — сколько символов фрагмента храним (для списка закладок).
const maxBookmarkExcerpt = 300

// LastBookmark возвращает последнюю добавленную закладку пользователя (по всем
// книгам). Прочитанные книги не берём. Если закладок нет — отдаёт null.
func (h *Handlers) LastBookmark(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	out, err := h.App.Bookmarks.Last(context.Background(), sessData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить последнюю закладку"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// ListBookmarks возвращает закладки книги в порядке по тексту.
func (h *Handlers) ListBookmarks(c *httpkit.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if !h.App.Bookmarks.BookOwned(context.Background(), sessData.Username, bookID) {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Книга не найдена"})
		return
	}

	out, err := h.App.Bookmarks.List(context.Background(), sessData.Username, bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить закладки"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// CreateBookmark сохраняет закладку на выделенном фрагменте.
func (h *Handlers) CreateBookmark(c *httpkit.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	if !h.App.Bookmarks.BookOwned(context.Background(), sessData.Username, bookID) {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Книга не найдена"})
		return
	}

	var req struct {
		Anchor  *int   `json:"anchor"`
		Excerpt string `json:"excerpt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Anchor == nil || *req.Anchor < 0 {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректная позиция закладки"})
		return
	}
	excerpt := req.Excerpt
	if strings.TrimSpace(excerpt) == "" {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Выделите текст для закладки"})
		return
	}
	if runes := []rune(excerpt); len(runes) > maxBookmarkExcerpt {
		excerpt = string(runes[:maxBookmarkExcerpt])
	}

	b, err := h.App.Bookmarks.Create(context.Background(), sessData.Username, bookID, *req.Anchor, excerpt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить закладку"})
		return
	}
	c.JSON(http.StatusOK, b)
}

// DeleteBookmark удаляет закладку книги.
func (h *Handlers) DeleteBookmark(c *httpkit.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID книги"})
		return
	}
	bookmarkID, err := strconv.Atoi(c.Param("bookmarkId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID закладки"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)
	ok, err := h.App.Bookmarks.Delete(context.Background(), sessData.Username, bookID, bookmarkID)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Закладка не найдена"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
