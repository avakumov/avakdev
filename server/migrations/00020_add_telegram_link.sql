-- +goose Up
-- Привязка Telegram: chat_id пользователя (после /start у бота) и
-- одноразовый код привязки (deep link t.me/<bot>?start=<code>).
ALTER TABLE users
	ADD COLUMN IF NOT EXISTS telegram_chat_id  TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS telegram_link_code TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS telegram_link_at   TIMESTAMP;

-- +goose Down
ALTER TABLE users
	DROP COLUMN IF EXISTS telegram_chat_id,
	DROP COLUMN IF EXISTS telegram_link_code,
	DROP COLUMN IF EXISTS telegram_link_at;
