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

const (
	// CookieName — имя cookie с токеном сессии (общее для main и handlers).
	CookieName = "avakumov_session"
	// SessionTTL — время жизни сессии.
	SessionTTL = 24 * time.Hour
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

	Users     *store.Users
	Reading   *store.Reading
	Drafts    *store.Drafts
	Bookmarks *store.Bookmarks
	Reports   *store.Reports
	Feed      *store.Feed
	Books     *store.Books
	Day       *store.Day

	// NotifDB — SQL уведомлений; Notifications — in-memory кэш поверх NotifDB.
	NotifDB       *store.Notifications
	Notifications *NotificationCache

	// Knowledge — in-memory хранилище конспектов знаний.
	Knowledge *NoteStore

	// Tasks и Goals — in-memory хранилища задач и целей (ссылаются друг на друга).
	Tasks *TaskStore
	Goals *GoalStore

	// Important — in-memory хранилище «важных» сообщений.
	Important *ImportantStore

	// UserMetrics — in-memory хранилище метрик пользователей.
	UserMetrics *MetricStore

	// AppTasks — in-memory хранилище задач по модификации приложения.
	AppTasks *AppTaskStore

	// Profile — in-memory хранилище профиля (резюме и фото).
	Profile *ProfileStore
}

// New собирает приложение на готовом пуле БД (nil — БД не настроена).
func New(db *pgxpool.Pool) *App {
	a := &App{DB: db}
	if db != nil {
		a.Sess = NewSessionStore()
		a.Users = store.NewUsers(db)
		a.Reading = store.NewReading(db)
		a.Drafts = store.NewDrafts(db)
		a.Bookmarks = store.NewBookmarks(db)
		a.Reports = store.NewReports(db)
		a.Feed = store.NewFeed(db)
		a.Books = store.NewBooks(db)
		a.Day = store.NewDay(db)
		a.NotifDB = store.NewNotifications(db)
	}
	return a
}

// InitNotifications создаёт in-memory кэш уведомлений и подгружает их из БД.
// Вызывается после New; без БД создаёт пустой кэш.
func (a *App) InitNotifications() error {
	var backend *store.Notifications
	if a.DB != nil {
		backend = a.NotifDB
	}
	c, err := NewNotificationCache(backend)
	if err != nil {
		return err
	}
	a.Notifications = c
	return nil
}

// InitKnowledge создаёт in-memory хранилище конспектов и подгружает их из БД.
// Вызывается после New; без БД создаёт пустое хранилище.
func (a *App) InitKnowledge() error {
	var pool *pgxpool.Pool
	if a.DB != nil {
		pool = a.DB
	}
	ns, err := NewNoteStore(pool)
	if err != nil {
		return err
	}
	a.Knowledge = ns
	return nil
}

// InitTasksGoals создаёт in-memory хранилища задач и целей, связывает их
// взаимными ссылками и подгружает данные из БД. Вызывается после New.
func (a *App) InitTasksGoals() error {
	if a.DB == nil {
		return nil
	}
	a.Tasks = NewTaskStore(a.DB)
	a.Goals = NewGoalStore(a.DB)
	a.Tasks.goals = a.Goals
	a.Goals.tasks = a.Tasks
	if err := a.Tasks.Load(); err != nil {
		return err
	}
	return a.Goals.Load()
}

// InitImportant создаёт in-memory хранилище «важных» сообщений и подгружает их
// из БД. Вызывается после New; без БД создаёт пустое хранилище.
func (a *App) InitImportant() error {
	var pool *pgxpool.Pool
	if a.DB != nil {
		pool = a.DB
	}
	s, err := NewImportantStore(pool)
	if err != nil {
		return err
	}
	a.Important = s
	return nil
}

// InitUserMetrics создаёт in-memory хранилище метрик и подгружает их из БД.
// Вызывается после New; без БД создаёт пустое хранилище.
func (a *App) InitUserMetrics() error {
	var pool *pgxpool.Pool
	if a.DB != nil {
		pool = a.DB
	}
	s, err := NewMetricStore(pool)
	if err != nil {
		return err
	}
	a.UserMetrics = s
	return nil
}

// InitAppTasks создаёт in-memory хранилище задач по модификации приложения.
// Вызывается после New; без БД создаёт пустое хранилище.
func (a *App) InitAppTasks() error {
	var pool *pgxpool.Pool
	if a.DB != nil {
		pool = a.DB
	}
	s, err := NewAppTaskStore(pool)
	if err != nil {
		return err
	}
	a.AppTasks = s
	return nil
}

// InitProfile создаёт in-memory хранилище профиля и подгружает данные из БД.
// Вызывается после New; без БД создаёт пустой профиль.
func (a *App) InitProfile() error {
	var pool *pgxpool.Pool
	if a.DB != nil {
		pool = a.DB
	}
	s, err := NewProfileStore(pool)
	if err != nil {
		return err
	}
	a.Profile = s
	return nil
}
