-- +goose Up
-- Раздел «Заметки»: быстрые записи-черновики пользователя (один текст на запись).
-- Отдельно от «Знаний» (knowledge_notes — конспекты с повторениями).
CREATE TABLE IF NOT EXISTS draft_notes (
    id       SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    content  TEXT NOT NULL DEFAULT '',
    created  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS draft_notes_username_idx ON draft_notes (username, id DESC);

-- +goose Down
DROP TABLE IF EXISTS draft_notes;
