-- +goose Up
-- Раздел «Чтение»: закладки в книгах. Позиция хранится в символах от начала
-- текста книги (как считает JS: единицы UTF-16), рядом — сохранённый фрагмент.
CREATE TABLE IF NOT EXISTS book_bookmarks (
    id       SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    anchor   INTEGER NOT NULL,
    excerpt  TEXT NOT NULL DEFAULT '',
    created  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS book_bookmarks_user_book_idx
    ON book_bookmarks (username, book_id, anchor);

-- +goose Down
DROP TABLE IF EXISTS book_bookmarks;
