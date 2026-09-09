-- +goose Up
-- Скорость чтения пользователя (символов в минуту) для расчёта времени
-- повторения заметок в разделе «День». 0 = среднее значение (по умолчанию).
ALTER TABLE users ADD COLUMN IF NOT EXISTS reading_speed INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS reading_speed;
