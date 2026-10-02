-- +goose Up
-- Объяснение и примеры к вопросу ленты. Показывается по кнопке «подробнее…»
-- под ответом; текст в Markdown (там уместны примеры кода).
-- Поле необязательное; у уже созданных элементов — пустая строка.
ALTER TABLE feed_items ADD COLUMN IF NOT EXISTS details TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE feed_items DROP COLUMN IF EXISTS details;
