-- +goose Up
-- Раздел «День»: ежедневный план пользователя.
CREATE TABLE IF NOT EXISTS day_plans (
    id             SERIAL PRIMARY KEY,
    username       TEXT NOT NULL,
    day            DATE NOT NULL,
    budget_minutes INT  NOT NULL DEFAULT 0,
    created        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (username, day)
);

-- Состав дня: задачи ('task') и заметки к повторению ('note').
-- minutes фиксируется в момент формирования плана (время уже посчитано).
CREATE TABLE IF NOT EXISTS day_items (
    id       SERIAL PRIMARY KEY,
    plan_id  INT NOT NULL REFERENCES day_plans (id) ON DELETE CASCADE,
    kind     TEXT NOT NULL, -- 'task' | 'note'
    ref_id   INT NOT NULL,
    minutes  INT NOT NULL DEFAULT 0,
    done     BOOLEAN NOT NULL DEFAULT false,
    position INT NOT NULL DEFAULT 0,
    UNIQUE (plan_id, kind, ref_id)
);
CREATE INDEX IF NOT EXISTS idx_day_items_plan ON day_items (plan_id);

-- +goose Down
DROP TABLE IF EXISTS day_items;
DROP TABLE IF EXISTS day_plans;
