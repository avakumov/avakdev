-- +goose Up
-- Текстовый отчёт дня переезжает в строку дня (day_plans.report): у каждого
-- пользователя и даты уже есть своя строка плана, отдельная таблица reports
-- больше не нужна.
ALTER TABLE day_plans ADD COLUMN IF NOT EXISTS report TEXT NOT NULL DEFAULT '';

-- Переносим старые глобальные отчёты. Раньше отчёт был один на дату и его
-- видели все пользователи — чтобы сохранить видимость, кладём текст каждому
-- пользователю (при отсутствии строки дня создаём её с пустым планом).
INSERT INTO day_plans (username, day, budget_minutes, report)
SELECT u.username, r.date, 0, r.content
FROM reports r
CROSS JOIN users u
WHERE r.content <> ''
ON CONFLICT (username, day)
DO UPDATE SET report = EXCLUDED.report;

DROP TABLE IF EXISTS reports;

-- +goose Down
-- Возвращаем таблицу reports (переносим тексты по одному на дату — выбрать
-- владельца из per-user хранения невозможно, берём любую непустую строку).
CREATE TABLE IF NOT EXISTS reports (
    date    DATE NOT NULL UNIQUE,
    content TEXT NOT NULL DEFAULT '',
    updated TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO reports (date, content)
SELECT DISTINCT ON (day) day, report
FROM day_plans
WHERE report <> '';

ALTER TABLE day_plans DROP COLUMN IF EXISTS report;
