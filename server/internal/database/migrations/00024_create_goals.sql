-- +goose Up
-- Раздел «Цели»: долгосрочные результаты с дедлайном, статусом и прогрессом.
CREATE TABLE IF NOT EXISTS goals (
	id          SERIAL PRIMARY KEY,
	username    TEXT NOT NULL,
	title       TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	target_date DATE,
	status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'achieved', 'cancelled')),
	progress    INT NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
	created     TIMESTAMP NOT NULL DEFAULT now(),
	updated     TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS goals;
