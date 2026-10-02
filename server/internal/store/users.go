package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// User — запись таблицы users.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
	// Phone — необязательный телефон пользователя.
	Phone string `json:"phone"`
	// Telegram — необязательное имя пользователя в Telegram (без @).
	Telegram string `json:"telegram"`
	// ReadingSpeed — скорость чтения (символов в минуту); 0 = «не задано».
	ReadingSpeed int `json:"reading_speed"`
	// CodeTheme — тема оформления блоков кода (id семейства).
	CodeTheme string `json:"code_theme"`
	// AvatarPreset — выбранный готовый вариант аватара (id) или пусто.
	AvatarPreset string `json:"avatar_preset"`
	// AvatarData — своё фото аватара в base64 (без data URI префикса).
	AvatarData string `json:"avatar_data"`
	// AvatarMime — MIME-тип фото аватара.
	AvatarMime string `json:"avatar_mime"`
	// TelegramChatID — chat_id привязанного Telegram (наружу не отдаём).
	TelegramChatID string `json:"-"`
	// Password хранится ТОЛЬКО внутри структуры, наружу никогда не уходит.
	Password string `json:"-"`
}

// Users — хранилище пользователей (таблица users).
type Users struct{ pool *pgxpool.Pool }

// NewUsers создаёт хранилище пользователей.
func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

// ByUsername возвращает пользователя по имени (без пароля).
func (s *Users) ByUsername(ctx context.Context, username string) (User, bool) {
	if s.pool == nil {
		return User{}, false
	}
	var u User
	err := s.pool.QueryRow(ctx,
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

// Credentials возвращает пользователя для входа (id, username, password,
// email, is_admin).
func (s *Users) Credentials(ctx context.Context, username string) (User, bool) {
	if s.pool == nil {
		return User{}, false
	}
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password, email, is_admin FROM users WHERE username = $1`,
		username).Scan(&u.ID, &u.Username, &u.Password, &u.Email, &u.IsAdmin)
	if err != nil {
		return User{}, false
	}
	return u, true
}

// UpdateContacts сохраняет контактные данные; readingSpeed/codeTheme — nil,
// если поле не трогали (тогда остаётся текущее значение в БД).
func (s *Users) UpdateContacts(ctx context.Context, username, phone, telegram string, readingSpeed *int, codeTheme *string) error {
	if s.pool == nil {
		return ErrNoDB
	}
	var speedArg, themeArg any
	if readingSpeed != nil {
		speedArg = *readingSpeed
	}
	if codeTheme != nil {
		themeArg = *codeTheme
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE users
		    SET phone = $1, telegram = $2,
		        reading_speed = COALESCE($3::int, reading_speed),
		        code_theme = COALESCE($4::text, code_theme)
		  WHERE username = $5`,
		phone, telegram, speedArg, themeArg, username)
	return err
}

// SetAvatar сохраняет аватар пользователя (preset + фото в base64).
func (s *Users) SetAvatar(ctx context.Context, username, preset, photo, mime string) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE users
		 SET avatar_preset = $1, avatar_data = $2, avatar_mime = $3
		 WHERE username = $4`,
		preset, photo, mime, username)
	return err
}
