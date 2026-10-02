-- +goose Up
-- Контактные данные пользователя (необязательные): телефон и Telegram.
ALTER TABLE users
	ADD COLUMN IF NOT EXISTS phone    TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS telegram TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users
	DROP COLUMN IF EXISTS phone,
	DROP COLUMN IF EXISTS telegram;
