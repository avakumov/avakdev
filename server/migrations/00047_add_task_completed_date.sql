-- +goose Up
-- Дата выполнения задачи (локальная дата, без времени). По ней задача попадает
-- в план/отчёты нужного дня: пользователь может отметить забытую задачу
-- задним числом. completed_at остаётся точным временем отметки.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS completed_date DATE;

-- Уже закрытые задачи переносим из completed_at. Пояса клиента задним числом
-- нет — берём дату по UTC (для записей около полуночи возможен сдвиг на сутки).
UPDATE tasks SET completed_date = (completed_at AT TIME ZONE 'UTC')::date
WHERE completed_at IS NOT NULL AND completed_date IS NULL;

-- +goose Down
ALTER TABLE tasks DROP COLUMN IF EXISTS completed_date;
