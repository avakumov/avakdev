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

// Notifications — SQL уведомлений: «входящие» колокольчика и рассылка по
// расписанию (таблицы notification_inbox и user_notifications). In-memory часть
// (кэш уведомлений) остаётся в main.
type Notifications struct{ pool *pgxpool.Pool }

// NewNotifications создаёт хранилище SQL уведомлений.
func NewNotifications(pool *pgxpool.Pool) *Notifications { return &Notifications{pool: pool} }

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
