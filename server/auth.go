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

	"avakumov/server/internal/httpkit"

	"github.com/jackc/pgx/v5/pgxpool"

	"avakumov/server/internal/database"
)

// User — представление записи в таблице users.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
	// Phone — необязательный телефон пользователя.
	Phone string `json:"phone"`
	// Telegram — необязательное имя пользователя в Telegram (без @).
	Telegram string `json:"telegram"`
	// ReadingSpeed — скорость чтения (символов в минуту); 0 = «не задано»
	// (при сохранении приводится к среднему значению 1500).
	ReadingSpeed int `json:"reading_speed"`
	// CodeTheme — тема оформления блоков кода (id семейства, напр. night-owl).
	// Светлый/тёмный вариант выбирается по теме сайта.
	CodeTheme string `json:"code_theme"`
	// AvatarPreset — выбранный готовый вариант аватара (id) или пусто.
	AvatarPreset string `json:"avatar_preset"`
	// AvatarData — своё фото аватара в base64 (без data URI префикса).
	AvatarData string `json:"avatar_data"`
	// AvatarMime — MIME-тип фото аватара (например image/jpeg).
	AvatarMime string `json:"avatar_mime"`
	// TelegramChatID — chat_id привязанного Telegram (не показываем наружу).
	TelegramChatID string `json:"-"`
	// Password хранится ТОЛЬКО внутри структуры, наружу никогда не уходит.
	Password string `json:"-"`
}

// userPayload — безопасное представление пользователя для JSON-ответов
// (пароль не включается никогда).
func userPayload(u User) httpkit.H {
	return httpkit.H{
		"username":        u.Username,
		"email":           u.Email,
		"is_admin":        u.IsAdmin,
		"sections":        userSections(u.IsAdmin),
		"phone":           u.Phone,
		"telegram":        u.Telegram,
		"reading_speed":   u.ReadingSpeed,
		"code_theme":      u.CodeTheme,
		"avatar_preset":   u.AvatarPreset,
		"avatar_data":     u.AvatarData,
		"avatar_mime":     u.AvatarMime,
		"telegram_linked": u.TelegramChatID != "",
	}
}

// Разделы приложения и кто их видит. Это ЕДИНСТВЕННОЕ место, где описана
// видимость пунктов меню: сервер сам сообщает клиенту список доступных
// разделов (поле sections в /api/me), а фронтенд лишь отрисовывает их.
// Сами разделы по-прежнему защищены на маршрутах (adminRequired).
var (
	authedSections = []string{
		"day", "goals", "tasks", "reports", "knowledge",
		"reading", "metrics", "profile", "important", "notes", "feed-edit",
		"user",
	}
	adminSections = []string{"server", "app", "db"}
)

// userSections возвращает разделы, доступные пользователю с ролью isAdmin.
func userSections(isAdmin bool) []string {
	out := make([]string, 0, len(authedSections)+len(adminSections))
	out = append(out, authedSections...)
	if isAdmin {
		out = append(out, adminSections...)
	}
	return out
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
// БД не настроена (пустая переменная) — приложение работает без неё.
func initDB() error {
	if err := database.Init(context.Background(), os.Getenv("DATABASE_URL")); err != nil {
		return err
	}
	// Ссылка на пул для хендлеров. Постепенно её вытесняют хранилища (store).
	db = database.Pool()
	if db != nil {
		sess = newSessionStore()
	}
	return nil
}

// authRequired — middleware, требующий активной сессии. Кладёт session
// в контекст запроса, чтобы её видели нижележащие middleware и обработчики.
func authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			httpkit.WriteJSON(w, http.StatusServiceUnavailable, httpkit.H{
				"error": "Авторизация отключена: база данных не настроена.",
			})
			return
		}
		token, err := r.Cookie(cookieName)
		if err != nil {
			httpkit.WriteJSON(w, http.StatusUnauthorized, httpkit.H{"error": "Требуется вход"})
			return
		}
		sessData, ok := sess.get(token.Value)
		if !ok {
			httpkit.WriteJSON(w, http.StatusUnauthorized, httpkit.H{"error": "Сессия истекла. Войдите снова."})
			return
		}
		next.ServeHTTP(w, httpkit.SetRequestValue(r, "session", sessData))
	})
}

// adminRequired — middleware, требующий прав администратора.
func adminRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, _ := httpkit.NewContext(w, r).Get("session")
		sessData, ok := v.(session)
		if !ok || !sessData.isAdmin {
			httpkit.WriteJSON(w, http.StatusForbidden, httpkit.H{"error": "Доступ только для администраторов"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loadUser возвращает пользователя по имени (без пароля — он не заполняется
// в этой выборке). Ошибка "no rows" возвращается как nil-структура + false.
func loadUser(username string) (User, bool) {
	var u User
	err := db.QueryRow(context.Background(),
		`SELECT id, username, email, is_admin, phone, telegram,
		        reading_speed, code_theme,
		        avatar_preset, avatar_data, avatar_mime, telegram_chat_id
		 FROM users WHERE username = $1`,
		username).Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin, &u.Phone,
		&u.Telegram, &u.ReadingSpeed, &u.CodeTheme, &u.AvatarPreset, &u.AvatarData,
		&u.AvatarMime, &u.TelegramChatID)
	if err != nil {
		return User{}, false
	}
	return u, true
}

// handleLogin аутентифицирует пользователя по username/password.
func handleLogin(c *httpkit.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Логин и пароль обязательны"})
		return
	}

	if db == nil {
		c.JSON(http.StatusServiceUnavailable, httpkit.H{"error": "Авторизация временно недоступна"})
		return
	}

	var u User
	err := db.QueryRow(context.Background(),
		`SELECT id, username, password, email, is_admin FROM users WHERE username = $1`,
		req.Username).Scan(&u.ID, &u.Username, &u.Password, &u.Email, &u.IsAdmin)
	if err != nil || u.Password != req.Password {
		c.JSON(http.StatusUnauthorized, httpkit.H{"error": "Неверный логин или пароль"})
		return
	}

	token, err := sess.create(&u, sessionTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось создать сессию"})
		return
	}
	// HttpOnly + SameSite — защита от XSS/CSRF; cookie отдаётся только на api.
	c.SetCookie(cookieName, token, int(sessionTTL.Seconds()), "/api", "", false, true)

	c.JSON(http.StatusOK, httpkit.H{
		"username": u.Username,
		"email":    u.Email,
		"is_admin": u.IsAdmin,
	})
}

// handleLogout завершает сессию и удаляет cookie.
func handleLogout(c *httpkit.Context) {
	if sess != nil {
		if token, err := c.Cookie(cookieName); err == nil {
			sess.delete(token)
		}
	}
	c.SetCookie(cookieName, "", -1, "/api", "", false, true)
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}

// handleMe возвращает данные текущего пользователя (или 401).
func handleMe(c *httpkit.Context) {
	sessVal, ok := c.Get("session")
	if !ok {
		c.JSON(http.StatusUnauthorized, httpkit.H{"error": "Требуется вход"})
		return
	}
	sessData, _ := sessVal.(session)
	u, found := loadUser(sessData.username)
	if !found {
		// Пользователь удалён при живой сессии — считаем сессию недействительной.
		c.JSON(http.StatusUnauthorized, httpkit.H{"error": "Сессия истекла. Войдите снова."})
		return
	}
	c.JSON(http.StatusOK, userPayload(u))
}

// defaultCodeTheme — тема кода по умолчанию (совпадает с DEFAULT в миграции).
const defaultCodeTheme = "night-owl"

// validCodeThemes — допустимые значения users.code_theme (семейства тем).
var validCodeThemes = map[string]bool{
	"night-owl": true,
	"github":    true,
	"solarized": true,
	"one":       true,
	"plain":     true,
	"dracula":   true,
	"monokai":   true,
}

// handleUpdateMe сохраняет контактные данные текущего пользователя
// (необязательные поля phone и telegram).
func handleUpdateMe(c *httpkit.Context) {
	sessVal, ok := c.Get("session")
	if !ok {
		c.JSON(http.StatusUnauthorized, httpkit.H{"error": "Требуется вход"})
		return
	}
	sessData, _ := sessVal.(session)

	var req struct {
		Phone        string  `json:"phone"`
		Telegram     string  `json:"telegram"`
		ReadingSpeed *int    `json:"reading_speed"`
		CodeTheme    *string `json:"code_theme"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.Telegram = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Telegram), "@"))
	if len(req.Phone) > 32 || len(req.Telegram) > 64 {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Значение слишком длинное"})
		return
	}
	if req.ReadingSpeed != nil {
		speed := *req.ReadingSpeed
		// 0 или отрицательное = «не задано» → среднее значение по умолчанию.
		if speed < 1 {
			speed = 1500
		}
		// Диапазон ползунка в профиле: 500–3000 символов в минуту.
		if speed > 3000 {
			speed = 3000
		}
		if speed > 5000 {
			speed = 5000
		}
		req.ReadingSpeed = &speed
	}
	if req.CodeTheme != nil {
		theme := strings.TrimSpace(*req.CodeTheme)
		// Пусто → тема по умолчанию (как и DEFAULT в миграции).
		if theme == "" {
			theme = defaultCodeTheme
		}
		if !validCodeThemes[theme] {
			c.JSON(http.StatusBadRequest, httpkit.H{"error": "Неизвестная тема кода"})
			return
		}
		req.CodeTheme = &theme
	}

	// Необязательные поля (reading_speed, code_theme) приходят как nil, если их
	// не трогали — COALESCE оставляет текущее значение в БД.
	var speedArg, themeArg interface{}
	if req.ReadingSpeed != nil {
		speedArg = *req.ReadingSpeed
	}
	if req.CodeTheme != nil {
		themeArg = *req.CodeTheme
	}
	if _, err := db.Exec(context.Background(),
		`UPDATE users
		    SET phone = $1, telegram = $2,
		        reading_speed = COALESCE($3::int, reading_speed),
		        code_theme = COALESCE($4::text, code_theme)
		  WHERE username = $5`,
		req.Phone, req.Telegram, speedArg, themeArg, sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить профиль"})
		return
	}

	u, found := loadUser(sessData.username)
	if !found {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, userPayload(u))
}

// logAuthConfig печатает состояние подключения к БД при старте.
func logAuthConfig() {
	if db == nil {
		log.Println("AUTH: база данных не настроена (DATABASE_URL пуст). Авторизация отключена.")
		return
	}
	log.Println("AUTH: подключение к PostgreSQL установлено. Авторизация включена.")
}
