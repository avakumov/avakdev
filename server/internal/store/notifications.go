package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationInboxItem — запись «входящих»: уведомление, наступившее по
// расписанию и ещё не закрытое пользователем (колокольчик).
type NotificationInboxItem struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Created string `json:"created"`
}

// DueTelegramNotification — наступившее telegram-уведомление с chat_id получателя.
type DueTelegramNotification struct {
	ID     int
	Text   string
	Type   string
	Unit   string
	Value  int
	DueAt  string
	ChatID string
}

// UserNotification — уведомление пользователя (создаёт себе сам).
type UserNotification struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Text        string `json:"text"`
	Type        string `json:"type"`
	DueAt       string `json:"due_at"` // RFC3339 (UTC)
	PeriodUnit  string `json:"period_unit"`
	PeriodValue int    `json:"period_value"`
	Channel     string `json:"channel"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// Notifications — SQL уведомлений: «входящие» колокольчика и рассылка по
// расписанию (таблицы notification_inbox и user_notifications).
type Notifications struct{ pool *pgxpool.Pool }

// NewNotifications создаёт хранилище SQL уведомлений.
func NewNotifications(pool *pgxpool.Pool) *Notifications { return &Notifications{pool: pool} }

// ListAll возвращает все уведомления (для загрузки кэша при старте).
func (s *Notifications) ListAll(ctx context.Context) ([]UserNotification, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,
		        username,
		        text,
		        type,
		        to_char(due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        period_unit,
		        period_value,
		        channel,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM user_notifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserNotification, 0)
	for rows.Next() {
		var n UserNotification
		if err := rows.Scan(&n.ID, &n.Username, &n.Text, &n.Type,
			&n.DueAt, &n.PeriodUnit, &n.PeriodValue, &n.Channel,
			&n.Created, &n.Updated); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Create вставляет уведомление и возвращает его с заполненными id/created/updated.
func (s *Notifications) Create(ctx context.Context, n UserNotification) (UserNotification, error) {
	if s.pool == nil {
		return UserNotification{}, ErrNoDB
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_notifications
		   (username, text, type, due_at, period_unit, period_value, channel)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id,
		           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		n.Username, n.Text, n.Type, n.DueAt, n.PeriodUnit, n.PeriodValue, n.Channel).
		Scan(&n.ID, &n.Created, &n.Updated)
	return n, err
}

// DeliverDueApp кладёт во «входящие» наступившие уведомления канала app
// (по одному на уведомление, пока оно не закрыто).
func (s *Notifications) DeliverDueApp(ctx context.Context) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO notification_inbox (username, notif_id, text)
		 SELECT n.username, n.id, n.text
		 FROM user_notifications n
		 WHERE n.channel = 'app'
		   AND n.due_at <= now()
		   AND NOT EXISTS (SELECT 1 FROM notification_inbox i WHERE i.notif_id = n.id)`)
	return err
}

// Inbox возвращает «входящие» пользователя (наступившие и не закрытые).
func (s *Notifications) Inbox(ctx context.Context, username string) ([]NotificationInboxItem, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,
		        text,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notification_inbox
		 WHERE username = $1
		 ORDER BY created DESC`,
		username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]NotificationInboxItem, 0)
	for rows.Next() {
		var it NotificationInboxItem
		if err := rows.Scan(&it.ID, &it.Text, &it.Created); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// DismissInbox удаляет «входящее» пользователя и возвращает id родительского
// уведомления. Если записи не было — ошибка.
func (s *Notifications) DismissInbox(ctx context.Context, username string, id int) (int, error) {
	if s.pool == nil {
		return 0, ErrNoDB
	}
	var notifID int
	err := s.pool.QueryRow(ctx,
		`DELETE FROM notification_inbox WHERE id = $1 AND username = $2
		 RETURNING notif_id`,
		id, username).Scan(&notifID)
	return notifID, err
}

// Meta возвращает параметры родительского уведомления (тип, due_at, период).
func (s *Notifications) Meta(ctx context.Context, username string, notifID int) (ntype, dueAt, unit string, value int, err error) {
	if s.pool == nil {
		return "", "", "", 0, ErrNoDB
	}
	err = s.pool.QueryRow(ctx,
		`SELECT type,
		        to_char(due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        period_unit, period_value
		 FROM user_notifications WHERE id = $1 AND username = $2`,
		notifID, username).Scan(&ntype, &dueAt, &unit, &value)
	return
}

// DeleteByID удаляет уведомление по id.
func (s *Notifications) DeleteByID(ctx context.Context, id int) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM user_notifications WHERE id = $1`, id)
	return err
}

// SetDue сдвигает ближайшее срабатывание уведомления.
func (s *Notifications) SetDue(ctx context.Context, id int, dueAt string) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE user_notifications SET due_at = $2, updated = now() WHERE id = $1`,
		id, dueAt)
	return err
}

// DueTelegram возвращает наступившие telegram-уведомления с chat_id получателя.
func (s *Notifications) DueTelegram(ctx context.Context) ([]DueTelegramNotification, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT n.id, n.text, n.type, n.period_unit, n.period_value,
		        to_char(n.due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        u.telegram_chat_id
		 FROM user_notifications n
		 JOIN users u ON u.username = n.username
		 WHERE n.channel = 'telegram'
		   AND n.due_at <= now()
		   AND u.telegram_chat_id <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DueTelegramNotification, 0)
	for rows.Next() {
		var n DueTelegramNotification
		if err := rows.Scan(&n.ID, &n.Text, &n.Type, &n.Unit, &n.Value,
			&n.DueAt, &n.ChatID); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}
