-- +goose Up
-- Раздел «Чтение»: хранение PDF как есть (без конвертации). Байты лежат
-- отдельно от books, чтобы список книг не тянул тяжёлый файл. Число страниц
-- сообщает клиент (pdf.js) и сохраняет в books.pages.
CREATE TABLE IF NOT EXISTS book_files (
    book_id INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    data    BYTEA NOT NULL,
    mime    TEXT NOT NULL DEFAULT 'application/pdf'
);

ALTER TABLE books ADD COLUMN IF NOT EXISTS pages INTEGER NOT NULL DEFAULT 0;

-- +goose Down
DROP TABLE IF EXISTS book_files;
ALTER TABLE books DROP COLUMN IF EXISTS pages;
