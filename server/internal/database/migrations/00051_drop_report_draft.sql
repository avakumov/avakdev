-- +goose Up
-- Черновик отчёта больше не нужен: текст отчёта сохраняется автоматически
-- прямо в day_plans.report (см. раздел «День»).
ALTER TABLE day_plans DROP COLUMN IF EXISTS report_draft;

-- +goose Down
ALTER TABLE day_plans ADD COLUMN IF NOT EXISTS report_draft TEXT NOT NULL DEFAULT '';
