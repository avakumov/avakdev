-- +goose Up
-- Черновик текстового отчёта дня (day_plans.report_draft). Пишется
-- автоматически при вводе в «Дне», чтобы текст не терялся при обновлении
-- страницы или работе с другого устройства. Хранится отдельно от report —
-- «боевого» отчёта, который сохраняется кнопкой.
ALTER TABLE day_plans ADD COLUMN IF NOT EXISTS report_draft TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE day_plans DROP COLUMN IF EXISTS report_draft;
