-- +goose Up
-- Периодичность уведомлений становится универсальной: единица измерения
-- (минуты/часы/дни/месяцы/годы) и значение вместо жёстких period_minutes.
ALTER TABLE user_notifications DROP COLUMN IF EXISTS period_minutes;
ALTER TABLE user_notifications
	ADD COLUMN period_unit  TEXT NOT NULL DEFAULT 'minute' CHECK (period_unit IN ('minute', 'hour', 'day', 'month', 'year')),
	ADD COLUMN period_value INT NOT NULL DEFAULT 1 CHECK (period_value >= 1);

-- +goose Down
ALTER TABLE user_notifications DROP COLUMN IF EXISTS period_unit;
ALTER TABLE user_notifications DROP COLUMN IF EXISTS period_value;
ALTER TABLE user_notifications ADD COLUMN period_minutes INT NOT NULL DEFAULT 0 CHECK (period_minutes >= 0);
