-- +goose Up
-- Раздел «Чтение»: цветные выделения текста в книгах. Диапазон хранится в
-- символах от начала текста книги (как считает JS: единицы UTF-16) — так же,
-- как позиции закладок; color — id палитры (yellow|green|blue|pink).
CREATE TABLE IF NOT EXISTS book_highlights (
    id           SERIAL PRIMARY KEY,
    username     TEXT NOT NULL,
    book_id      INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    start_offset INTEGER NOT NULL,
    end_offset   INTEGER NOT NULL,
    color        TEXT NOT NULL,
    excerpt      TEXT NOT NULL DEFAULT '',
    created      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS book_highlights_user_book_idx
    ON book_highlights (username, book_id, start_offset);

-- +goose Down
DROP TABLE IF EXISTS book_highlights;
