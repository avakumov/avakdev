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

// importantMessage — «важное» сообщение одного пользователя.
type importantMessage struct {
	Content   string
	UpdatedBy string
	UpdatedAt string
}

// importantStore — хранилище «важных» сообщений: у каждого пользователя
// своё сообщение (ключ — username), отметки о прочтении тоже персональные
// (один раз в сутки).
type importantStore struct {
	mu       sync.Mutex
	messages map[string]importantMessage // username -> сообщение
	hasDB    bool
	// seen: username -> дата последнего прочтения (YYYY-MM-DD, UTC).
	seen map[string]string
}

// important — глобальное хранилище «важных» сообщений.
var important *importantStore

// initImportant инициализирует глобальное хранилище «важных» сообщений.
// При наличии БД подгружает сохранённые данные в память.
// (Таблица создаётся версионированными миграциями goose, см. migrations/.)
func initImportant() error {
	important = &importantStore{
		hasDB:    db != nil,
		messages: make(map[string]importantMessage),
		seen:     make(map[string]string),
	}
	if !important.hasDB {
		return nil
	}

	// Подгружаем сообщения пользователей в память.
	rows, err := db.Query(context.Background(),
		`SELECT username,
		        content,
		        updated_by,
		        to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM important_message`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var username string
		var msg importantMessage
		if err := rows.Scan(&username, &msg.Content, &msg.UpdatedBy, &msg.UpdatedAt); err != nil {
			return err
		}
		important.messages[username] = msg
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Подгружаем отметки о прочтении в память.
	seenRows, err := db.Query(context.Background(),
		`SELECT username, to_char(seen_on, 'YYYY-MM-DD') FROM important_seen`)
	if err != nil {
		return err
	}
	defer seenRows.Close()
	for seenRows.Next() {
		var username, seenOn string
		if err := seenRows.Scan(&username, &seenOn); err != nil {
			return err
		}
		important.seen[username] = seenOn
	}
	return seenRows.Err()
}

// get возвращает «важное» сообщение пользователя.
// ok=false означает, что пользователь ещё не создавал своё сообщение.
func (s *importantStore) get(username string) (importantMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg, ok := s.messages[username]
	return msg, ok
}

// seenToday возвращает true, если пользователь уже прочитал сообщение сегодня.
func (s *importantStore) seenToday(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	return s.seen[username] == today
}

// save сохраняет «важное» сообщение пользователя (создаёт или обновляет).
func (s *importantStore) save(username, content string) (importantMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := importantMessage{
		Content:   content,
		UpdatedBy: username,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`INSERT INTO important_message (username, content, updated_by, updated_at)
			 VALUES ($1, $2, $3, now())
			 ON CONFLICT (username)
			 DO UPDATE SET content = $2, updated_by = $3, updated_at = now()`,
			username, content, username); err != nil {
			return importantMessage{}, err
		}
	}

	s.messages[username] = msg
	return msg, nil
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

// handleGetImportant отдаёт «важное» сообщение текущего пользователя:
// текст, автора и время последнего обновления, а также флаг enabled
// (показ только на production) и seen_today (показывается раз в сутки).
func handleGetImportant(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	msg, _ := important.get(sessData.username)
	c.JSON(http.StatusOK, gin.H{
		"enabled":    importantEnabled(),
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
		"seen_today": important.seenToday(sessData.username),
	})
}

// handleSaveImportant сохраняет «важное» сообщение текущего пользователя.
// Каждый авторизованный пользователь управляет только своим сообщением.
func handleSaveImportant(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	msg, err := important.save(sessData.username, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить сообщение"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":    msg.Content,
		"updated_by": msg.UpdatedBy,
		"updated_at": msg.UpdatedAt,
	})
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
