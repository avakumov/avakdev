-- +goose Up
-- Раздел «Чтение»: отметка о прочтении книги. NULL — книга не прочитана,
-- прочитанные книги уезжают в конец списка и не предлагаются для чтения в «Дне».
ALTER TABLE books ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE books DROP COLUMN IF EXISTS finished_at;
