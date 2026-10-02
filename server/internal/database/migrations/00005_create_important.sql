-- +goose Up
CREATE TABLE IF NOT EXISTS important_message (
	id         INTEGER PRIMARY KEY,
	content    TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS important_seen (
	username TEXT PRIMARY KEY,
	seen_on  DATE NOT NULL
);

-- Единственное «важное» сообщение (общее для всех пользователей).
INSERT INTO important_message (id, content)
VALUES (1, '')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS important_seen;
DROP TABLE IF EXISTS important_message;
