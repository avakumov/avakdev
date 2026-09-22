-- +goose Up
-- Раздел «Лента»: элементы ленты пользователя. Первый тип контента —
-- «вопрос-ответ» (kind = 'qa'): вопрос, ответ и счётчик показов.
-- Ленту наполняют в разделе меню «Лента», читают — свайпом (мобильные).
CREATE TABLE IF NOT EXISTS feed_items (
    id       SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    kind     TEXT NOT NULL DEFAULT 'qa',
    question TEXT NOT NULL DEFAULT '',
    answer   TEXT NOT NULL DEFAULT '',
    views    INTEGER NOT NULL DEFAULT 0,
    created  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS feed_items_username_idx ON feed_items (username, id DESC);

-- +goose Down
DROP TABLE IF EXISTS feed_items;
