-- +goose Up
-- Раздел (область) элемента ленты: короткое слово вроде «golang», «linux», «ооп».
-- Нужен, чтобы по короткому вопросу было понятно, из какой он области.
-- Поле обязательное для новых элементов; у уже созданных — пустая строка.
ALTER TABLE feed_items ADD COLUMN IF NOT EXISTS topic TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE feed_items DROP COLUMN IF EXISTS topic;
