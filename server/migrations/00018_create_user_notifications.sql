-- +goose Up
-- Уведомления пользователя: текст, тип (однократно/периодически),
-- ближайшее срабатывание, период (для периодических) и канал доставки.
-- Telegram-доставка пока заглушка (channel='telegram'), реально показываем
-- только во встроенных уведомлениях (channel='app').
CREATE TABLE IF NOT EXISTS user_notifications (
	id             SERIAL PRIMARY KEY,
	username       TEXT NOT NULL,
	text           TEXT NOT NULL,
	type           TEXT NOT NULL DEFAULT 'once' CHECK (type IN ('once', 'periodic')),
	due_at         TIMESTAMP NOT NULL,
	period_minutes INT NOT NULL DEFAULT 0 CHECK (period_minutes >= 0),
	channel        TEXT NOT NULL DEFAULT 'app' CHECK (channel IN ('app', 'telegram')),
	created        TIMESTAMP NOT NULL DEFAULT now(),
	updated        TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS user_notifications;
