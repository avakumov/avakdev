-- +goose Up
-- Для агента по задачам: колонка с результатом выполнения и статус «failed».
ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS result TEXT NOT NULL DEFAULT '';

ALTER TABLE app_tasks
	DROP CONSTRAINT IF EXISTS app_tasks_status_check;

ALTER TABLE app_tasks
	ADD CONSTRAINT app_tasks_status_check
	CHECK (status IN ('new', 'in_progress', 'done', 'cancelled', 'failed'));

-- +goose Down
ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS result;

ALTER TABLE app_tasks
	DROP CONSTRAINT IF EXISTS app_tasks_status_check;

ALTER TABLE app_tasks
	ADD CONSTRAINT app_tasks_status_check
	CHECK (status IN ('new', 'in_progress', 'done', 'cancelled'));
