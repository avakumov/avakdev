package handlers

import (
	"context"
	"net/http"
	"strings"

	"avakumov/server/internal/app"
	"avakumov/server/internal/store"
)

// userPayload — безопасное представление пользователя для JSON-ответов
// (пароль не включается никогда).
func userPayload(u store.User) map[string]any {
	return map[string]any{
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
// Сами разделы по-прежнему защищены на маршрутах (AdminRequired).
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

// userByUsername возвращает пользователя по имени (без пароля).
func (h *Handlers) userByUsername(username string) (store.User, bool) {
	return h.App.Users.ByUsername(context.Background(), username)
}

// AuthRequired — middleware, требующий активной сессии. Кладёт session
// в контекст запроса, чтобы её видели нижележащие middleware и обработчики.
func (h *Handlers) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.App.DB == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"error": "Авторизация отключена: база данных не настроена.",
			})
			return
		}
		token, err := r.Cookie(app.CookieName)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Требуется вход"})
			return
		}
		sessData, ok := h.App.Sess.Get(token.Value)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Сессия истекла. Войдите снова."})
			return
		}
		next.ServeHTTP(w, withSession(r, sessData))
	})
}

// AdminRequired — middleware, требующий прав администратора.
func (h *Handlers) AdminRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := sessionOf(r)
		if !ok || !v.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "Доступ только для администраторов"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Login аутентифицирует пользователя по username/password.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Логин и пароль обязательны"})
		return
	}

	if h.App.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Авторизация временно недоступна"})
		return
	}

	u, ok := h.App.Users.Credentials(context.Background(), req.Username)
	if !ok || u.Password != req.Password {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Неверный логин или пароль"})
		return
	}

	token, err := h.App.Sess.Create(u.Username, u.IsAdmin, app.SessionTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось создать сессию"})
		return
	}
	// HttpOnly + SameSite — защита от XSS/CSRF; cookie отдаётся только на api.
	http.SetCookie(w, &http.Cookie{
		Name:     app.CookieName,
		Value:    token,
		MaxAge:   int(app.SessionTTL.Seconds()),
		Path:     "/api",
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"username": u.Username,
		"email":    u.Email,
		"is_admin": u.IsAdmin,
	})
}

// Logout завершает сессию и удаляет cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if token, err := cookieValue(r, app.CookieName); err == nil {
		h.App.Sess.Delete(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     app.CookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/api",
		HttpOnly: true,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me возвращает данные текущего пользователя (или 401).
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	sessData, ok := sessionOf(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Требуется вход"})
		return
	}
	u, found := h.userByUsername(sessData.Username)
	if !found {
		// Пользователь удалён при живой сессии — считаем сессию недействительной.
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Сессия истекла. Войдите снова."})
		return
	}
	writeJSON(w, http.StatusOK, userPayload(u))
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

// UpdateMe сохраняет контактные данные текущего пользователя
// (необязательные поля phone и telegram).
func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	sessData, ok := sessionOf(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Требуется вход"})
		return
	}

	var req struct {
		Phone        string  `json:"phone"`
		Telegram     string  `json:"telegram"`
		ReadingSpeed *int    `json:"reading_speed"`
		CodeTheme    *string `json:"code_theme"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.Telegram = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Telegram), "@"))
	if len(req.Phone) > 32 || len(req.Telegram) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Значение слишком длинное"})
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
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Неизвестная тема кода"})
			return
		}
		req.CodeTheme = &theme
	}

	// Необязательные поля (reading_speed, code_theme) приходят как nil, если их
	// не трогали — COALESCE в store оставляет текущее значение в БД.
	if err := h.App.Users.UpdateContacts(context.Background(), sessData.Username,
		req.Phone, req.Telegram, req.ReadingSpeed, req.CodeTheme); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить профиль"})
		return
	}

	u, found := h.userByUsername(sessData.Username)
	if !found {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Пользователь не найден"})
		return
	}
	writeJSON(w, http.StatusOK, userPayload(u))
}
