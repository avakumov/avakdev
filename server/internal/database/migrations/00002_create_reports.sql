-- +goose Up
CREATE TABLE IF NOT EXISTS reports (
	date    DATE NOT NULL UNIQUE,
	content TEXT NOT NULL DEFAULT '',
	updated TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS reports;
