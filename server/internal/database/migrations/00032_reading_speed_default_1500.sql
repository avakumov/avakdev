-- +goose Up
-- Значение по умолчанию скорости чтения — 1500 символов в минуту (среднее).
-- Раньше 0 означал «не задано»; приводим такие записи к явному среднему
-- значению и меняем дефолт колонки для новых пользователей.
UPDATE users SET reading_speed = 1500 WHERE reading_speed <= 0;
ALTER TABLE users ALTER COLUMN reading_speed SET DEFAULT 1500;

-- +goose Down
-- Возвращаем дефолт 0 (значения пользователей не трогаем).
ALTER TABLE users ALTER COLUMN reading_speed SET DEFAULT 0;
