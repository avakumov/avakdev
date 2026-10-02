-- +goose Up
-- Задачи по модификации приложения (раздел «Приложение»).
-- Статус: новая / в работе / готова / отменена.
CREATE TABLE IF NOT EXISTS app_tasks (
	id          SERIAL PRIMARY KEY,
	username    TEXT NOT NULL,
	title       TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status      TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'in_progress', 'done', 'cancelled')),
	created     TIMESTAMP NOT NULL DEFAULT now(),
	updated     TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS app_tasks;
