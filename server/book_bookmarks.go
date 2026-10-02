package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"avakumov/server/internal/httpkit"
)

// Раздел «Чтение»: закладки в книгах. Пользователь выделяет текст в книге,
// клиент считает позицию выделения (в символах от начала текста книги) и
// сохраняет её вместе с фрагментом. По клику на закладку клиент находит это
// место в тексте заново — HTML книги не меняется, поэтому позиция устойчива.
// SQL живёт в store.Bookmarks (см. internal/store/bookmarks.go).

// maxBookmarkExcerpt — сколько символов фрагмента храним (для списка закладок).
const maxBookmarkExcerpt = 300

// handleLastBookmark возвращает последнюю добавленную закладку пользователя
// (по всем книгам) — с неё продолжается чтение. Прочитанные книги не берём:
// в «Дне» они больше не предлагаются. Если закладок нет, отдаёт null.
func handleLastBookmark(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(session)
	out, err := bookmarksStore.Last(context.Background(), sessData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить последнюю закладку"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleListBookmarks возвращает закладки книги в порядке по тексту.
func handleListBookmarks(c *httpkit.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if !bookmarksStore.BookOwned(context.Background(), sessData.Username, bookID) {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Книга не найдена"})
		return
	}

	out, err := bookmarksStore.List(context.Background(), sessData.Username, bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить закладки"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleCreateBookmark сохраняет закладку на выделенном фрагменте.
func handleCreateBookmark(c *httpkit.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if !bookmarksStore.BookOwned(context.Background(), sessData.Username, bookID) {
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

	b, err := bookmarksStore.Create(context.Background(), sessData.Username, bookID, *req.Anchor, excerpt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить закладку"})
		return
	}
	c.JSON(http.StatusOK, b)
}

// handleDeleteBookmark удаляет закладку книги.
func handleDeleteBookmark(c *httpkit.Context) {
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
	sessData, _ := c.MustGet("session").(session)
	ok, err := bookmarksStore.Delete(context.Background(), sessData.Username, bookID, bookmarkID)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Закладка не найдена"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
