-- +goose Up
-- Потраченное время задачи теперь складывается из фактического времени по дням
-- (day_items.actual_minutes), а не из отдельного поля tasks.actual_hours.
-- Переносим ранее введённое значение в самый ранний день, где задача есть в
-- плане, чтобы ничего не потерять. Задачи, которых нет ни в одном плане,
-- оставляют значение в tasks.actual_hours как «базу без разбивки по дням».
WITH target AS (
    SELECT DISTINCT ON (i.ref_id)
           i.id AS item_id,
           t.actual_hours
      FROM day_items i
      JOIN day_plans p ON p.id = i.plan_id
      JOIN tasks t ON t.id = i.ref_id AND t.username = p.username
     WHERE i.kind = 'task' AND t.actual_hours > 0 AND i.actual_minutes = 0
     ORDER BY i.ref_id, p.day, i.position
)
UPDATE day_items d
   SET actual_minutes = LEAST(1440, ROUND(g.actual_hours * 60))::int
  FROM target g
 WHERE d.id = g.item_id;

UPDATE tasks t
   SET actual_hours = 0
 WHERE t.actual_hours > 0
   AND EXISTS (
       SELECT 1
         FROM day_items i
         JOIN day_plans p ON p.id = i.plan_id
        WHERE i.kind = 'task' AND i.ref_id = t.id AND p.username = t.username
   );

-- +goose Down
-- Перенос обратно однозначно не восстановить: значения остались в
-- day_items.actual_minutes. Откат затрагивает только схему — изменений нет.
SELECT 1;
