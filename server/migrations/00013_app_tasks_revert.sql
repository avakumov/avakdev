-- +goose Up
-- Хэш коммита задачи, запрос отката и время отката.
ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS commit_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS revert_requested BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS reverted_at TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS commit_hash;

ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS revert_requested;

ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS reverted_at;
