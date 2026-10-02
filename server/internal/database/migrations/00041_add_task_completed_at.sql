-- +goose Up
-- Когда задача отмечена выполненной. Нужно отчёту дня: показать все задачи,
-- закрытые в этот день, даже если их не было в плане.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

-- У уже выполненных задач точного времени закрытия нет — берём время последнего
-- изменения как наиболее близкое к нему.
UPDATE tasks SET completed_at = updated WHERE status = 'done' AND completed_at IS NULL;

-- +goose Down
ALTER TABLE tasks DROP COLUMN IF EXISTS completed_at;
