package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Session — активная сессия пользователя: строка таблицы sessions плюс
// актуальные права из users.
type Session struct {
	Username string
	IsAdmin  bool
	Expires  time.Time
}

// Sessions — хранилище сессий (таблица sessions). Ключ — sha256-хэш токена:
// в БД сырой токен не хранится, он живёт только в cookie клиента.
type Sessions struct{ pool *pgxpool.Pool }

// NewSessions создаёт хранилище сессий.
func NewSessions(pool *pgxpool.Pool) *Sessions { return &Sessions{pool: pool} }

// hashToken — sha256(token) в hex. Токен — 32 случайных байта, поэтому быстрого
// sha256 достаточно (в отличие от пароля, где нужен bcrypt).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken генерирует новый случайный токен сессии (64 hex-символа).
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create генерирует токен, сохраняет его хэш со сроком жизни ttl и возвращает
// сырой токен (для cookie). Заодно подчищает истёкшие сессии.
func (s *Sessions) Create(ctx context.Context, username string, ttl time.Duration) (string, error) {
	if s.pool == nil {
		return "", ErrNoDB
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires < now()`); err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, username, expires) VALUES ($1, $2, $3)`,
		hashToken(token), username, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	return token, nil
}

// Get возвращает сессию по токену, если она существует и не истекла. Права
// (is_admin) берутся актуальными из users: удалили пользователя — JOIN не
// найдёт строку, и сессия считается недействительной.
func (s *Sessions) Get(ctx context.Context, token string) (Session, bool) {
	if s.pool == nil || token == "" {
		return Session{}, false
	}
	var sess Session
	err := s.pool.QueryRow(ctx,
		`SELECT s.username, u.is_admin, s.expires
		   FROM sessions s
		   JOIN users u ON u.username = s.username
		  WHERE s.token_hash = $1 AND s.expires > now()`,
		hashToken(token)).Scan(&sess.Username, &sess.IsAdmin, &sess.Expires)
	if err != nil {
		return Session{}, false
	}
	return sess, true
}

// Delete удаляет сессию по токену (logout).
func (s *Sessions) Delete(ctx context.Context, token string) {
	if s.pool == nil || token == "" {
		return
	}
	_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
}
