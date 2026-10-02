-- +goose Up
-- Цель чтения на день: своя для каждой даты (по умолчанию — 1 час).
-- По умолчанию цель задаётся только этой колонкой, чтобы значение всегда
-- было в одном месте.
ALTER TABLE book_reading ADD COLUMN IF NOT EXISTS goal_seconds INTEGER NOT NULL DEFAULT 3600;

-- +goose Down
ALTER TABLE book_reading DROP COLUMN IF EXISTS goal_seconds;
