package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Раздел «Заметки»: быстрые записи-черновики. В коде называются draft, чтобы не
// путать с конспектами раздела «Знания» (knowledge_notes). Текст один — либо
// пишем с нуля в плавающем окне, либо правим уже сохранённую запись.

// DraftNote — заметка пользователя.
type DraftNote struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// maxDraftRunes — предельный размер текста заметки (символов).
const maxDraftRunes = 20000

// Единый формат времени заметки (RFC3339, UTC).
const (
	draftCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	draftUpdatedExpr = `to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
)

// handleListDrafts возвращает заметки пользователя: свежие сверху.
func handleListDrafts(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	rows, err := db.Query(context.Background(),
		`SELECT id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr+`
		 FROM draft_notes WHERE username = $1
		 ORDER BY updated DESC, id DESC`,
		sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить заметки"})
		return
	}
	defer rows.Close()

	out := make([]DraftNote, 0)
	for rows.Next() {
		var n DraftNote
		if err := rows.Scan(&n.ID, &n.Content, &n.Created, &n.Updated); err == nil {
			out = append(out, n)
		}
	}
	c.JSON(http.StatusOK, out)
}

// handleCreateDraft сохраняет новую заметку.
func handleCreateDraft(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var n DraftNote
	err = db.QueryRow(context.Background(),
		`INSERT INTO draft_notes (username, content)
		 VALUES ($1, $2)
		 RETURNING id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr,
		sessData.username, content).
		Scan(&n.ID, &n.Content, &n.Created, &n.Updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить заметку"})
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
func handleUpdateDraft(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID заметки"})
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	content, err := draftContent(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var n DraftNote
	err = db.QueryRow(context.Background(),
		`UPDATE draft_notes SET content = $3, updated = now()
		 WHERE id = $1 AND username = $2
		 RETURNING id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr,
		id, sessData.username, content).
		Scan(&n.ID, &n.Content, &n.Created, &n.Updated)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleDeleteDraft удаляет заметку.
func handleDeleteDraft(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID заметки"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	tag, err := db.Exec(context.Background(),
		`DELETE FROM draft_notes WHERE id = $1 AND username = $2`, id, sessData.username)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Заметка не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
