-- +goose Up
-- Время срабатывания и «входящие» храним как абсолютные моменты (timestamptz),
-- чтобы значения не зависели от часового пояса сессии БД. Старые данные
-- трактуем как ранее сохранённые UTC (по ним строились to_char ... AT TIME ZONE 'UTC').
ALTER TABLE user_notifications
	ALTER COLUMN due_at TYPE timestamptz USING due_at AT TIME ZONE 'UTC';
ALTER TABLE notification_inbox
	ALTER COLUMN created TYPE timestamptz USING created AT TIME ZONE 'UTC';

-- +goose Down
ALTER TABLE user_notifications
	ALTER COLUMN due_at TYPE timestamp USING due_at AT TIME ZONE 'UTC';
ALTER TABLE notification_inbox
	ALTER COLUMN created TYPE timestamp USING created AT TIME ZONE 'UTC';
