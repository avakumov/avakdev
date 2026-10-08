-- +goose Up
-- Для PDF подсветка привязана к странице: page — номер страницы (1-based),
-- start_offset/end_offset — смещения в тексте этой страницы. Для fb2/epub
-- page = 0, а смещения по-прежнему отсчитываются от начала всей книги.
ALTER TABLE book_highlights ADD COLUMN IF NOT EXISTS page INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE book_highlights DROP COLUMN IF EXISTS page;
