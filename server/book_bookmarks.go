package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Раздел «Чтение»: закладки в книгах. Пользователь выделяет текст в книге,
// клиент считает позицию выделения (в символах от начала текста книги) и
// сохраняет её вместе с фрагментом. По клику на закладку клиент находит это
// место в тексте заново — HTML книги не меняется, поэтому позиция устойчива.

// BookBookmark — закладка книги.
type BookBookmark struct {
	ID      int    `json:"id"`
	Anchor  int    `json:"anchor"`
	Excerpt string `json:"excerpt"`
	Created string `json:"created"`
}

// maxBookmarkExcerpt — сколько символов фрагмента храним (для списка закладок).
const maxBookmarkExcerpt = 300

// bookBookmarkCreatedExpr — единый формат времени создания закладки (RFC3339, UTC).
const bookBookmarkCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// bookmarkBookAccess проверяет, что книга существует и принадлежит пользователю.
func bookmarkBookAccess(bookID int, username string) bool {
	var one int
	err := db.QueryRow(context.Background(),
		`SELECT 1 FROM books WHERE id = $1 AND username = $2`, bookID, username).Scan(&one)
	return err == nil
}

// handleLastBookmark возвращает последнюю добавленную закладку пользователя
// (по всем книгам) — с неё продолжается чтение. Прочитанные книги не берём:
// в «Дне» они больше не предлагаются. Если закладок нет, отдаёт null.
func handleLastBookmark(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)

	var out struct {
		BookID    int    `json:"book_id"`
		BookTitle string `json:"book_title"`
		Anchor    int    `json:"anchor"`
		Excerpt   string `json:"excerpt"`
	}
	err := db.QueryRow(context.Background(),
		`SELECT bm.book_id, b.title, bm.anchor, bm.excerpt
		 FROM book_bookmarks bm
		 JOIN books b ON b.id = bm.book_id AND b.username = bm.username
		 WHERE bm.username = $1 AND b.finished_at IS NULL
		 ORDER BY bm.created DESC, bm.id DESC
		 LIMIT 1`,
		sessData.username).
		Scan(&out.BookID, &out.BookTitle, &out.Anchor, &out.Excerpt)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, nil)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить последнюю закладку"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// handleListBookmarks возвращает закладки книги в порядке по тексту.
func handleListBookmarks(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if !bookmarkBookAccess(bookID, sessData.username) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Книга не найдена"})
		return
	}

	rows, err := db.Query(context.Background(),
		`SELECT id, anchor, excerpt, `+bookBookmarkCreatedExpr+`
		 FROM book_bookmarks
		 WHERE username = $1 AND book_id = $2
		 ORDER BY anchor, id`,
		sessData.username, bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить закладки"})
		return
	}
	defer rows.Close()

	out := make([]BookBookmark, 0)
	for rows.Next() {
		var b BookBookmark
		if err := rows.Scan(&b.ID, &b.Anchor, &b.Excerpt, &b.Created); err == nil {
			out = append(out, b)
		}
	}
	c.JSON(http.StatusOK, out)
}

// handleCreateBookmark сохраняет закладку на выделенном фрагменте.
func handleCreateBookmark(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if !bookmarkBookAccess(bookID, sessData.username) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Книга не найдена"})
		return
	}

	var req struct {
		Anchor  *int   `json:"anchor"`
		Excerpt string `json:"excerpt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Anchor == nil || *req.Anchor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректная позиция закладки"})
		return
	}
	excerpt := req.Excerpt
	if strings.TrimSpace(excerpt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Выделите текст для закладки"})
		return
	}
	if runes := []rune(excerpt); len(runes) > maxBookmarkExcerpt {
		excerpt = string(runes[:maxBookmarkExcerpt])
	}

	var b BookBookmark
	err = db.QueryRow(context.Background(),
		`INSERT INTO book_bookmarks (username, book_id, anchor, excerpt)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, anchor, excerpt, `+bookBookmarkCreatedExpr,
		sessData.username, bookID, *req.Anchor, excerpt).
		Scan(&b.ID, &b.Anchor, &b.Excerpt, &b.Created)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить закладку"})
		return
	}
	c.JSON(http.StatusOK, b)
}

// handleDeleteBookmark удаляет закладку книги.
func handleDeleteBookmark(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	bookmarkID, err := strconv.Atoi(c.Param("bookmarkId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID закладки"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	tag, err := db.Exec(context.Background(),
		`DELETE FROM book_bookmarks WHERE id = $1 AND book_id = $2 AND username = $3`,
		bookmarkID, bookID, sessData.username)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Закладка не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
