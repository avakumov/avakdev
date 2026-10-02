// Package app — домен приложения: пул БД, сессии, хранилища (store) и общие
// типы. HTTP-слой (internal/handlers) и main обращаются сюда.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"avakumov/server/internal/store"
)

// Session — активная сессия пользователя.
type Session struct {
	Username string
	IsAdmin  bool
	Expires  time.Time
}

// SessionStore — in-memory хранилище активных сессий.
type SessionStore struct {
	mu   sync.Mutex
	data map[string]Session
}

// NewSessionStore создаёт хранилище сессий.
func NewSessionStore() *SessionStore {
	return &SessionStore{data: make(map[string]Session)}
}

// Create добавляет новую сессию и возвращает её токен.
func (s *SessionStore) Create(username string, isAdmin bool, ttl time.Duration) (string, error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tok)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token] = Session{Username: username, IsAdmin: isAdmin, Expires: time.Now().Add(ttl)}
	return token, nil
}

// Get возвращает сессию по токену, если она существует и не истекла.
func (s *SessionStore) Get(token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data[token]
	if !ok {
		return Session{}, false
	}
	if time.Now().After(sess.Expires) {
		delete(s.data, token)
		return Session{}, false
	}
	return sess, true
}

// Delete удаляет сессию по токену (logout).
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, token)
}

// App — общие зависимости приложения: пул БД, сессии и хранилища доменов.
type App struct {
	DB   *pgxpool.Pool
	Sess *SessionStore

	Reading   *store.Reading
	Drafts    *store.Drafts
	Bookmarks *store.Bookmarks
	Reports   *store.Reports
}

// New собирает приложение на готовом пуле БД (nil — БД не настроена).
func New(db *pgxpool.Pool) *App {
	a := &App{DB: db}
	if db != nil {
		a.Sess = NewSessionStore()
		a.Reading = store.NewReading(db)
		a.Drafts = store.NewDrafts(db)
		a.Bookmarks = store.NewBookmarks(db)
		a.Reports = store.NewReports(db)
	}
	return a
}
