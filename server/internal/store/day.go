package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DayItem — позиция сохранённого дня.
type DayItem struct {
	Kind    string `json:"kind"` // task | note
	RefID   int    `json:"ref_id"`
	Title   string `json:"title"`
	Meta    string `json:"meta"` // категория задачи / тема заметки
	Minutes int    `json:"minutes"`
	Done    bool   `json:"done"`
	// SpentMinutes — фактически потраченное время позиции в этот день.
	SpentMinutes int `json:"spent_minutes"`
}

// CompletedTask — задача, закрытая в этот день (для отчёта дня).
type CompletedTask struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	DoneAt   string `json:"done_at"` // RFC3339, UTC
	// CompletedDate — за какой день задача выполнена (ГГГГ-ММ-ДД).
	CompletedDate string `json:"completed_date"`
	// ActualHours — потраченное время задачи (ч); PlannedHours — плановая оценка.
	ActualHours  float64 `json:"actual_hours"`
	PlannedHours float64 `json:"planned_hours"`
}

// DayBody — единый ответ для дня.
type DayBody struct {
	Date          string    `json:"date"`
	BudgetMinutes int       `json:"budget_minutes"`
	TotalMinutes  int       `json:"total_minutes"`
	Items         []DayItem `json:"items"`
	// Report — сохранённый текст отчёта за день.
	Report string `json:"report"`
	// CompletedTasks — все задачи, отмеченные выполненными в этот день.
	CompletedTasks []CompletedTask `json:"completed_tasks"`
}

// DaySummary — строка истории (прошедшие дни).
type DaySummary struct {
	Date          string `json:"date"`
	BudgetMinutes int    `json:"budget_minutes"`
	TotalMinutes  int    `json:"total_minutes"`
	Tasks         int    `json:"tasks"`
	Notes         int    `json:"notes"`
	// SpentMinutes — фактически проставленное время дня по задачам.
	SpentMinutes int `json:"spent_minutes"`
	// HasPlan — был ли сохранён план на этот день.
	HasPlan bool `json:"has_plan"`
}

// SaveItem — позиция плана для сохранения (минуты уже посчитаны вызывающим).
type SaveItem struct {
	Kind    string
	RefID   int
	Minutes int
}

// Day — хранилище планов дня (таблицы day_plans и day_items).
type Day struct{ pool *pgxpool.Pool }

// NewDay создаёт хранилище дня.
func NewDay(pool *pgxpool.Pool) *Day { return &Day{pool: pool} }

// ReadingSpeed возвращает скорость чтения пользователя (users.reading_speed).
func (s *Day) ReadingSpeed(ctx context.Context, username string) (int, bool) {
	if s.pool == nil {
		return 0, false
	}
	var v int
	if err := s.pool.QueryRow(ctx,
		`SELECT reading_speed FROM users WHERE username = $1`, username).Scan(&v); err != nil {
		return 0, false
	}
	return v, true
}

// PlanInfo возвращает шапку сохранённого дня. ok=false — плана нет.
func (s *Day) PlanInfo(ctx context.Context, username, day string) (planID, budget int, report string, ok bool) {
	if s.pool == nil {
		return 0, 0, "", false
	}
	err := s.pool.QueryRow(ctx,
		`SELECT id, budget_minutes, COALESCE(report, '')
		 FROM day_plans
		 WHERE username = $1 AND day = $2`, username, day).
		Scan(&planID, &budget, &report)
	if err != nil {
		return 0, 0, "", false
	}
	return planID, budget, report, true
}

// Items возвращает позиции плана (без title/meta — их дополняет вызывающий).
func (s *Day) Items(ctx context.Context, planID int) ([]DayItem, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT kind, ref_id, minutes, done, position, actual_minutes
		 FROM day_items WHERE plan_id = $1 ORDER BY position`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DayItem, 0)
	for rows.Next() {
		var it DayItem
		var pos int
		if err := rows.Scan(&it.Kind, &it.RefID, &it.Minutes, &it.Done, &pos, &it.SpentMinutes); err != nil {
			continue
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// CompletedTasks возвращает задачи пользователя, закрытые в указанный день.
func (s *Day) CompletedTasks(ctx context.Context, username, day string) []CompletedTask {
	out := make([]CompletedTask, 0)
	if s.pool == nil {
		return out
	}
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.title, t.category,
		        to_char(t.completed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        COALESCE(to_char(t.completed_date,'YYYY-MM-DD'),''),
		        COALESCE((SELECT SUM(i.actual_minutes)
		                    FROM day_items i
		                    JOIN day_plans p ON p.id = i.plan_id
		                   WHERE i.kind = 'task' AND i.ref_id = t.id
		                     AND p.username = t.username), 0)::float8 / 60.0,
		        t.planned_hours
		   FROM tasks t
		  WHERE t.username = $1 AND t.status = 'done' AND t.completed_date = $2::date
		  ORDER BY t.completed_at`,
		username, day)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t CompletedTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.DoneAt, &t.CompletedDate, &t.ActualHours, &t.PlannedHours); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// SavePlan сохраняет (перезаписывает) план дня в одной транзакции. Факт и
// отметка «выполнено» у остающихся позиций сохраняются (пере-формирование дня
// не обнуляет уже введённое время).
func (s *Day) SavePlan(ctx context.Context, username, day string, budget int, items []SaveItem) error {
	if s.pool == nil {
		return ErrNoDB
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var planID int
	if err := tx.QueryRow(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (username, day)
		 DO UPDATE SET budget_minutes = EXCLUDED.budget_minutes, updated = now()
		 RETURNING id`,
		username, day, budget).Scan(&planID); err != nil {
		return err
	}

	type kv struct {
		kind string
		id   int
	}
	type prevItem struct {
		actual int
		done   bool
	}
	prev := make(map[kv]prevItem)
	if rows, err := tx.Query(ctx,
		`SELECT kind, ref_id, actual_minutes, done FROM day_items WHERE plan_id = $1`, planID); err == nil {
		for rows.Next() {
			var k kv
			var p prevItem
			if err := rows.Scan(&k.kind, &k.id, &p.actual, &p.done); err == nil {
				prev[k] = p
			}
		}
		rows.Close()
	}

	if _, err := tx.Exec(ctx, `DELETE FROM day_items WHERE plan_id = $1`, planID); err != nil {
		return err
	}
	for pos, it := range items {
		p := prev[kv{it.Kind, it.RefID}]
		if _, err := tx.Exec(ctx,
			`INSERT INTO day_items (plan_id, kind, ref_id, minutes, done, position, actual_minutes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			planID, it.Kind, it.RefID, it.Minutes, p.done, pos, p.actual); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SetItemDone отмечает позицию плана выполненной (или снимает отметку).
func (s *Day) SetItemDone(ctx context.Context, username, day, kind string, refID int, done bool) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE day_items SET done = $1
		 WHERE kind = $2 AND ref_id = $3 AND plan_id =
		       (SELECT id FROM day_plans WHERE username = $4 AND day = $5)`,
		done, kind, refID, username, day)
	return err
}

// EnsurePlan создаёт строку дня при необходимости и возвращает её id.
func (s *Day) EnsurePlan(ctx context.Context, username, day string) (int, error) {
	if s.pool == nil {
		return 0, ErrNoDB
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, 0)
		 ON CONFLICT (username, day) DO NOTHING`, username, day); err != nil {
		return 0, err
	}
	var planID int
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM day_plans WHERE username = $1 AND day = $2`,
		username, day).Scan(&planID)
	return planID, err
}

// SetItemSpent создаёт/обновляет позицию дня и её фактическое время. Задача с
// указанным временем должна появиться в «Плане дня», поэтому позиция создаётся
// при необходимости (позиция в конец, плановые минуты — planned).
func (s *Day) SetItemSpent(ctx context.Context, planID, refID, minutes, planned int) error {
	if s.pool == nil {
		return ErrNoDB
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO day_items (plan_id, kind, ref_id, minutes, position, actual_minutes)
		 VALUES ($1, 'task', $2, $3,
		         COALESCE((SELECT MAX(position) + 1 FROM day_items WHERE plan_id = $1), 0),
		         $4)
		 ON CONFLICT (plan_id, kind, ref_id)
		 DO UPDATE SET actual_minutes = EXCLUDED.actual_minutes`,
		planID, refID, planned, minutes)
	return err
}

// History возвращает дни (сначала новые): сохранённые планы и дни, в которые
// закрывались задачи.
func (s *Day) History(ctx context.Context, username string) ([]DaySummary, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`WITH plan_days AS (
		     SELECT p.day AS day,
		            p.budget_minutes,
		            COALESCE(SUM(i.minutes), 0) AS total_minutes,
		            COUNT(*) FILTER (WHERE i.kind = 'task') AS tasks,
		            COUNT(*) FILTER (WHERE i.kind = 'note') AS notes
		     FROM day_plans p
		     LEFT JOIN day_items i ON i.plan_id = p.id
		     WHERE p.username = $1
		     GROUP BY p.id, p.day, p.budget_minutes
		 ),
		 done_days AS (
		     SELECT t.completed_date AS day
		     FROM tasks t
		     WHERE t.username = $1 AND t.status = 'done' AND t.completed_date IS NOT NULL
		     GROUP BY 1
		 ),
		 spent_days AS (
		     SELECT p.day AS day,
		            COALESCE(SUM(i.actual_minutes), 0)::int AS task_minutes
		       FROM day_plans p
		       JOIN day_items i ON i.plan_id = p.id AND i.kind = 'task'
		      WHERE p.username = $1
		      GROUP BY p.day
		 )
		 SELECT to_char(d.day, 'YYYY-MM-DD'),
		        COALESCE(pd.budget_minutes, 0),
		        COALESCE(pd.total_minutes, 0),
		        COALESCE(pd.tasks, 0),
		        COALESCE(pd.notes, 0),
		        COALESCE(sd.task_minutes, 0),
		        (pd.day IS NOT NULL)
		 FROM (SELECT day FROM plan_days UNION SELECT day FROM done_days) d
		 LEFT JOIN plan_days pd ON pd.day = d.day
		 LEFT JOIN spent_days sd ON sd.day = d.day
		 ORDER BY d.day DESC
		 LIMIT 90`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := make([]DaySummary, 0)
	for rows.Next() {
		var d DaySummary
		if err := rows.Scan(&d.Date, &d.BudgetMinutes, &d.TotalMinutes, &d.Tasks, &d.Notes, &d.SpentMinutes, &d.HasPlan); err == nil {
			days = append(days, d)
		}
	}
	return days, rows.Err()
}
