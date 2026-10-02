-- +goose Up
-- Раздел «Чтение»: книги пользователя (fb2/epub), сконвертированные в HTML.
CREATE TABLE IF NOT EXISTS books (
    id       SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    title    TEXT NOT NULL DEFAULT '',
    author   TEXT NOT NULL DEFAULT '',
    format   TEXT NOT NULL DEFAULT '',
    html     TEXT NOT NULL DEFAULT '',
    created  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS books_username_idx ON books (username, id DESC);

-- +goose Down
DROP TABLE IF EXISTS books;
