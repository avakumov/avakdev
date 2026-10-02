-- +goose Up
-- Кнопка «Deploy» у задачи: dev-агент видит запрос деплоя, коммитит
-- изменения и запускает make deploy.
ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS deploy_requested BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE app_tasks
	ADD COLUMN IF NOT EXISTS deployed_at TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS deploy_requested;

ALTER TABLE app_tasks
	DROP COLUMN IF EXISTS deployed_at;
