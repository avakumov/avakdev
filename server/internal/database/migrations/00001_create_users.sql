-- +goose Up
CREATE TABLE IF NOT EXISTS users (
	id       SERIAL PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	email    TEXT NOT NULL DEFAULT '',
	is_admin BOOLEAN NOT NULL DEFAULT false
);

-- +goose Down
DROP TABLE IF EXISTS users;
