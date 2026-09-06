-- +goose Up
-- Задача может относиться к цели (раздел «Цели»). При удалении цели
-- задачи остаются, но ссылка сбрасывается (SET NULL).
ALTER TABLE tasks ADD COLUMN goal_id INT;
ALTER TABLE tasks
	ADD CONSTRAINT tasks_goal_id_fkey FOREIGN KEY (goal_id)
		REFERENCES goals (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_goal_id_fkey;
ALTER TABLE tasks DROP COLUMN IF EXISTS goal_id;
