-- +goose Up
-- Реакции на элемент ленты: «знаю» / «не знаю». Счётчики накопительные —
-- растут каждый раз, когда пользователь отмечает элемент в ленте
-- (за один показ допустима одна реакция).
ALTER TABLE feed_items
    ADD COLUMN IF NOT EXISTS know_count    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unknown_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE feed_items
    DROP COLUMN IF EXISTS know_count,
    DROP COLUMN IF EXISTS unknown_count;
