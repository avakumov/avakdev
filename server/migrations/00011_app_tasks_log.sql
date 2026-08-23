-- +goose Up
-- Журнал выполнения задачи агентом (подробный лог инструментов и проверок).
ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS log TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS log;
