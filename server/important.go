package main

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// importantEnabled — «важное» сообщение показывается только на production.
// Продакшен определяется по GIN_MODE=release (задаётся в systemd-юните
// скриптом deploy.sh при установке на сервер).
func importantEnabled() bool {
	return os.Getenv("GIN_MODE") == "release"
}

// importantStore — хранилище «важного» сообщения и отметок о прочтении.
// Сообщение одно на всех (единственная строка id=1 в таблице important_message),
// отметки о прочтении — по пользователю: один раз в сутки.
type importantStore struct {
	mu        sync.Mutex
	content   string
	updatedBy string
	updatedAt string
	hasDB     bool
	// seen: username -> дата последнего прочтения (YYYY-MM-DD, UTC).
	seen map[string]string
}

// important — глобальное хранилище «важного» сообщения.
var important *importantStore

// initImportant инициализирует глобальное хранилище «важного» сообщения.
// При наличии БД подгружает сохранённые данные в память.
// (Таблицы и строка id=1 создаются миграциями goose, см. migrations/.)
func initImportant() error {
	important = &importantStore{
		hasDB: db != nil,
		seen:  make(map[string]string),
	}
	if !important.hasDB {
		return nil
	}

	var content, updatedBy, updatedAt string
	err := db.QueryRow(context.Background(),
		`SELECT content,
		        updated_by,
		        to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM important_message WHERE id = 1`).
		Scan(&content, &updatedBy, &updatedAt)
	if err != nil && err.Error() != "no rows in result set" {
		return err
	}
	important.content = content
	important.updatedBy = updatedBy
	important.updatedAt = updatedAt

	// Подгружаем отметки о прочтении в память.
	rows, err := db.Query(context.Background(),
		`SELECT username, to_char(seen_on, 'YYYY-MM-DD') FROM important_seen`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var username, seenOn string
		if err := rows.Scan(&username, &seenOn); err != nil {
			return err
		}
		important.seen[username] = seenOn
	}
	return rows.Err()
}

// getContent возвращает текст сообщения.
func (s *importantStore) getContent() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.content
}

// getUpdatedBy возвращает имя пользователя, который последним обновил сообщение.
func (s *importantStore) getUpdatedBy() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updatedBy
}

// getUpdatedAt возвращает время последнего обновления сообщения (RFC3339, UTC).
func (s *importantStore) getUpdatedAt() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updatedAt
}

// seenToday возвращает true, если пользователь уже прочитал сообщение сегодня.
func (s *importantStore) seenToday(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	return s.seen[username] == today
}

// save обновляет текст сообщения (вызывается администратором из меню «Важное»).
func (s *importantStore) save(content, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.content = content
	s.updatedBy = username
	s.updatedAt = time.Now().UTC().Format(time.RFC3339)

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE important_message
			 SET content = $1, updated_by = $2, updated_at = now()
			 WHERE id = 1`,
			content, username); err != nil {
			return err
		}
	}
	return nil
}

// markSeen отмечает, что пользователь прочитал сообщение сегодня.
// Повторный вызов в тот же день просто обновляет дату на ту же.
func (s *importantStore) markSeen(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	s.seen[username] = today

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`INSERT INTO important_seen (username, seen_on)
			 VALUES ($1, CURRENT_DATE)
			 ON CONFLICT (username) DO UPDATE SET seen_on = CURRENT_DATE`,
			username); err != nil {
			return err
		}
	}
	return nil
}

// handleGetImportant отдаёт «важное» сообщение: текст, автора и время
// последнего обновления, а также флаг enabled (показ только на production)
// и seen_today (показывается раз в сутки).
func handleGetImportant(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	c.JSON(http.StatusOK, gin.H{
		"enabled":    importantEnabled(),
		"content":    important.getContent(),
		"updated_by": important.getUpdatedBy(),
		"updated_at": important.getUpdatedAt(),
		"seen_today": important.seenToday(sessData.username),
	})
}

// handleSaveImportant сохраняет текст «важного» сообщения.
// Доступно только администраторам (роут в группе adminRequired).
func handleSaveImportant(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := important.save(req.Content, sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить сообщение"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleMarkImportantSeen отмечает, что текущий пользователь прочитал
// сообщение сегодня — до следующего дня оно ему больше не покажется.
func handleMarkImportantSeen(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	if err := important.markSeen(sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось отметить сообщение прочитанным"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
