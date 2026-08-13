package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User — представление записи в таблице users.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
	// Password хранится ТОЛЬКО внутри структуры, наружу никогда не уходит.
	Password string `json:"-"`
}

// session — активная сессия пользователя.
type session struct {
	username string
	isAdmin  bool
	expires  time.Time
}

// sessionStore — in-memory хранилище активных сессий.
type sessionStore struct {
	mu   sync.Mutex
	data map[string]session
}

func newSessionStore() *sessionStore {
	return &sessionStore{data: make(map[string]session)}
}

// create добавляет новую сессию и возвращает её токен.
func (s *sessionStore) create(u *User, ttl time.Duration) (string, error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tok)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token] = session{
		username: u.Username,
		isAdmin:  u.IsAdmin,
		expires:  time.Now().Add(ttl),
	}
	return token, nil
}

// get возвращает сессию по токену, если она существует и не истекла.
func (s *sessionStore) get(token string) (session, bool) {
	if token == "" {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data[token]
	if !ok {
		return session{}, false
	}
	if time.Now().After(sess.expires) {
		delete(s.data, token)
		return session{}, false
	}
	return sess, true
}

// delete удаляет сессию по токену (logout).
func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, token)
}

const (
	cookieName = "avakumov_session"
	sessionTTL = 24 * time.Hour
)

var (
	db   *pgxpool.Pool
	sess *sessionStore
)

// initDB подключается к PostgreSQL по строке подключения из DATABASE_URL.
func initDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil // БД не настроена — авторизация будет отключена.
	}
	var err error
	// Пул соединений. Внутри pgx сам разберётся с контекстом и переподключением.
	db, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(context.Background()); err != nil {
		return err
	}
	sess = newSessionStore()

	// При каждом старте проверить/создать таблицу users (идемпотентно).
	if _, err := db.Exec(context.Background(), createUsersTableSQL); err != nil {
		return err
	}
	return nil
}

const createUsersTableSQL = `
CREATE TABLE IF NOT EXISTS users (
	id       SERIAL PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	email    TEXT NOT NULL DEFAULT '',
	is_admin BOOLEAN NOT NULL DEFAULT false
);
`

// authRequired — middleware, требующий активной сессии.
func authRequired(c *gin.Context) {
	if db == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": "Авторизация отключена: база данных не настроена.",
		})
		return
	}
	token, err := c.Cookie(cookieName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Требуется вход"})
		return
	}
	sessData, ok := sess.get(token)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Сессия истекла. Войдите снова."})
		return
	}
	// Кладём информацию о пользователе в контекст для нижележащих обработчиков.
	c.Set("session", sessData)
	c.Next()
}

// adminRequired — middleware, требующий прав администратора.
func adminRequired(c *gin.Context) {
	sessData, ok := c.MustGet("session").(session)
	if !ok || !sessData.isAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Доступ только для администраторов"})
		return
	}
	c.Next()
}

// handleLogin аутентифицирует пользователя по username/password.
func handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Логин и пароль обязательны"})
		return
	}

	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Авторизация временно недоступна"})
		return
	}

	var u User
	err := db.QueryRow(context.Background(),
		`SELECT id, username, password, email, is_admin FROM users WHERE username = $1`,
		req.Username).Scan(&u.ID, &u.Username, &u.Password, &u.Email, &u.IsAdmin)
	if err != nil || u.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный логин или пароль"})
		return
	}

	token, err := sess.create(&u, sessionTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать сессию"})
		return
	}
	// HttpOnly + SameSite — защита от XSS/CSRF; cookie отдаётся только на api.
	c.SetCookie(cookieName, token, int(sessionTTL.Seconds()), "/api", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"username": u.Username,
		"email":    u.Email,
		"is_admin": u.IsAdmin,
	})
}

// handleLogout завершает сессию и удаляет cookie.
func handleLogout(c *gin.Context) {
	if sess != nil {
		if token, err := c.Cookie(cookieName); err == nil {
			sess.delete(token)
		}
	}
	c.SetCookie(cookieName, "", -1, "/api", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleMe возвращает данные текущего пользователя (или 401).
func handleMe(c *gin.Context) {
	sessVal, ok := c.Get("session")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется вход"})
		return
	}
	sessData, _ := sessVal.(session)
	c.JSON(http.StatusOK, gin.H{
		"username": sessData.username,
		"is_admin": sessData.isAdmin,
	})
}

// logAuthConfig печатает состояние подключения к БД при старте.
func logAuthConfig() {
	if db == nil {
		log.Println("AUTH: база данных не настроена (DATABASE_URL пуст). Авторизация отключена.")
		return
	}
	log.Println("AUTH: подключение к PostgreSQL установлено. Авторизация включена.")
}
