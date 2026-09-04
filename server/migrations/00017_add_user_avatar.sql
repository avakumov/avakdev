-- +goose Up
-- Аватар пользователя: готовый вариант (preset) или своё фото (base64 + MIME).
ALTER TABLE users
	ADD COLUMN IF NOT EXISTS avatar_preset TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS avatar_data   TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS avatar_mime   TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users
	DROP COLUMN IF EXISTS avatar_preset,
	DROP COLUMN IF EXISTS avatar_data,
	DROP COLUMN IF EXISTS avatar_mime;
