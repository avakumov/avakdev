-- +goose Up
-- Единица измерения для числовых метрик (кг, мин, км…).
-- Для boolean-метрик («да/нет») единица не нужна — остаётся пустой.
ALTER TABLE metric_definitions
	ADD COLUMN IF NOT EXISTS unit TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE metric_definitions
	DROP COLUMN IF EXISTS unit;
