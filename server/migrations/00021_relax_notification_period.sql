-- +goose Up
-- Однократные уведомления хранят период как пустую строку/0 — разрешаем это.
ALTER TABLE user_notifications DROP CONSTRAINT IF EXISTS user_notifications_period_unit_check;
ALTER TABLE user_notifications DROP CONSTRAINT IF EXISTS user_notifications_period_value_check;
ALTER TABLE user_notifications
	ADD CONSTRAINT user_notifications_period_unit_check
		CHECK (period_unit IN ('', 'minute', 'hour', 'day', 'month', 'year'));
ALTER TABLE user_notifications
	ADD CONSTRAINT user_notifications_period_value_check
		CHECK (period_value >= 0);

-- +goose Down
ALTER TABLE user_notifications DROP CONSTRAINT IF EXISTS user_notifications_period_unit_check;
ALTER TABLE user_notifications DROP CONSTRAINT IF EXISTS user_notifications_period_value_check;
ALTER TABLE user_notifications
	ADD CONSTRAINT user_notifications_period_unit_check
		CHECK (period_unit IN ('minute', 'hour', 'day', 'month', 'year'));
ALTER TABLE user_notifications
	ADD CONSTRAINT user_notifications_period_value_check
		CHECK (period_value >= 1);
