-- +goose Up
-- Фактически потраченное время по позиции дня (в минутах; 0 — не указано).
-- Храним отдельно от плановых minutes: одна и та же задача может стоять в
-- планах разных дней, и факт у каждого дня свой. При подсчёте «потрачено»
-- факт дня имеет приоритет над фактическим временем задачи (tasks.actual_hours).
ALTER TABLE day_items ADD COLUMN IF NOT EXISTS actual_minutes INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE day_items DROP COLUMN IF EXISTS actual_minutes;
