-- +goose Up
-- Раздел «Задачи»: категории задач, планируемое и фактическое время (часы),
-- дедлайн и статус. Владелец задачи — конкретный пользователь.
CREATE TABLE IF NOT EXISTS tasks (
	id            SERIAL PRIMARY KEY,
	username      TEXT NOT NULL,
	category      TEXT NOT NULL DEFAULT 'Прочее',
	title         TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	planned_hours NUMERIC NOT NULL DEFAULT 0,
	actual_hours  NUMERIC NOT NULL DEFAULT 0,
	deadline      DATE,
	status        TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done', 'cancelled')),
	created       TIMESTAMP NOT NULL DEFAULT now(),
	updated       TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS tasks;
