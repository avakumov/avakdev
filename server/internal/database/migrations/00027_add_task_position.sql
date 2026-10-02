-- +goose Up
-- Порядок выполнения задач внутри цели (последовательность).
-- 0 означает «вне цели»; внутри цели позиции идут с 1 по N.
ALTER TABLE tasks ADD COLUMN position INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_tasks_goal_position ON tasks (goal_id, position);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_goal_position;
ALTER TABLE tasks DROP COLUMN IF EXISTS position;
