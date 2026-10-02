-- +goose Up
CREATE TABLE IF NOT EXISTS profile (
	id          SERIAL PRIMARY KEY,
	description TEXT NOT NULL DEFAULT '',
	resume      TEXT NOT NULL DEFAULT '',
	photo       TEXT NOT NULL DEFAULT '',
	photo_mime  TEXT NOT NULL DEFAULT '',
	updated     TIMESTAMP NOT NULL DEFAULT now()
);

-- Колонки фото добавлены позже — на случай БД, созданных до этого момента.
ALTER TABLE profile
	ADD COLUMN IF NOT EXISTS photo      TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS photo_mime TEXT NOT NULL DEFAULT '';

-- Единственная строка профиля приложения.
INSERT INTO profile (id, description, resume, photo, photo_mime)
VALUES (1, '', '', '', '')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS profile;
