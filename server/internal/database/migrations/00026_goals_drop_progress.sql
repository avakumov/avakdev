-- +goose Up
-- Прогресс цели больше не хранится в БД: он вычисляется на лету из задач,
-- привязанных к цели (доля выполненных задач среди неотменённых).
ALTER TABLE goals DROP COLUMN IF EXISTS progress;

-- +goose Down
ALTER TABLE goals ADD COLUMN IF NOT EXISTS progress INT NOT NULL DEFAULT 0;
