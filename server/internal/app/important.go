package app

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportantEnabled — «важное» сообщение показывается только на production.
// Продакшен определяется по APP_ENV=production (задаётся в systemd-юните
// скриптом deploy.sh при установке на сервер).
func ImportantEnabled() bool {
	return os.Getenv("APP_ENV") == "production"
}

// ImportantMessage — «важное» сообщение одного пользователя.
type ImportantMessage struct {
	Content   string
	UpdatedBy string
	UpdatedAt string
}

// ImportantStore — хранилище «важных» сообщений: у каждого пользователя
// своё сообщение (ключ — username), отметки о прочтении тоже персональные
// (один раз в сутки).
type ImportantStore struct {
	mu       sync.Mutex
	messages map[string]ImportantMessage // username -> сообщение
	pool     *pgxpool.Pool
	// seen: username -> дата последнего прочтения (YYYY-MM-DD, UTC).
	seen map[string]string
}

// NewImportantStore создаёт хранилище и подгружает данные из БД
// (pool == nil — работаем без БД).
func NewImportantStore(pool *pgxpool.Pool) (*ImportantStore, error) {
	s := &ImportantStore{
		pool:     pool,
		messages: make(map[string]ImportantMessage),
		seen:     make(map[string]string),
	}
	if pool == nil {
		return s, nil
	}

	// Подгружаем сообщения пользователей в память.
	rows, err := pool.Query(context.Background(),
		`SELECT username,
		        content,
		        updated_by,
		        to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM important_message`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var username string
		var msg ImportantMessage
		if err := rows.Scan(&username, &msg.Content, &msg.UpdatedBy, &msg.UpdatedAt); err != nil {
			return nil, err
		}
		s.messages[username] = msg
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Подгружаем отметки о прочтении в память.
	seenRows, err := pool.Query(context.Background(),
		`SELECT username, to_char(seen_on, 'YYYY-MM-DD') FROM important_seen`)
	if err != nil {
		return nil, err
	}
	defer seenRows.Close()
	for seenRows.Next() {
		var username, seenOn string
		if err := seenRows.Scan(&username, &seenOn); err != nil {
			return nil, err
		}
		s.seen[username] = seenOn
	}
	return s, seenRows.Err()
}

// Get возвращает «важное» сообщение пользователя.
// ok=false означает, что пользователь ещё не создавал своё сообщение.
func (s *ImportantStore) Get(username string) (ImportantMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg, ok := s.messages[username]
	return msg, ok
}

// SeenToday возвращает true, если пользователь уже прочитал сообщение сегодня.
func (s *ImportantStore) SeenToday(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	return s.seen[username] == today
}

// Save сохраняет «важное» сообщение пользователя (создаёт или обновляет).
func (s *ImportantStore) Save(username, content string) (ImportantMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := ImportantMessage{
		Content:   content,
		UpdatedBy: username,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`INSERT INTO important_message (username, content, updated_by, updated_at)
			 VALUES ($1, $2, $3, now())
			 ON CONFLICT (username)
			 DO UPDATE SET content = $2, updated_by = $3, updated_at = now()`,
			username, content, username); err != nil {
			return ImportantMessage{}, err
		}
	}

	s.messages[username] = msg
	return msg, nil
}

// MarkSeen отмечает, что пользователь прочитал сообщение сегодня.
// Повторный вызов в тот же день просто обновляет дату на ту же.
func (s *ImportantStore) MarkSeen(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	s.seen[username] = today

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`INSERT INTO important_seen (username, seen_on)
			 VALUES ($1, CURRENT_DATE)
			 ON CONFLICT (username) DO UPDATE SET seen_on = CURRENT_DATE`,
			username); err != nil {
			return err
		}
	}
	return nil
}
