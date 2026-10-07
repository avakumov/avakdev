-- +goose Up
-- Сессии переносим в БД, чтобы они переживали деплой/перезапуск процесса.
-- Храним только sha256-хэш токена (сырой токен — в cookie), а is_admin не
-- дублируем: берём актуальное значение из users через JOIN.
CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    username   TEXT NOT NULL,
    expires    TIMESTAMPTZ NOT NULL,
    created    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_expires_idx ON sessions (expires);

-- +goose Down
DROP TABLE IF EXISTS sessions;
