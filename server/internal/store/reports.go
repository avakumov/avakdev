package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Report — текстовый отчёт за день пользователя. Хранится в day_plans.report
// (одна строка на пользователя и дату), поэтому отдельной таблицы нет.
type Report struct {
	Date    string `json:"date"`
	Content string `json:"content"`
	Updated string `json:"updated"`
}

// Reports — хранилище текстовых отчётов по дням (таблица day_plans).
type Reports struct{ pool *pgxpool.Pool }

// NewReports создаёт хранилище отчётов.
func NewReports(pool *pgxpool.Pool) *Reports { return &Reports{pool: pool} }

// List возвращает отчёты пользователя (только дни с непустым текстом), новые сверху.
func (s *Reports) List(ctx context.Context, username string) ([]Report, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT to_char(day,'YYYY-MM-DD'),
		        report,
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM day_plans
		 WHERE username = $1 AND report <> ''
		 ORDER BY day DESC`,
		username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Report, 0)
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.Date, &r.Content, &r.Updated); err == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// Upsert создаёт или обновляет текст отчёта за день. Если строки дня ещё нет —
// она создаётся с пустым планом (отчёт может быть без сохранённого плана).
func (s *Reports) Upsert(ctx context.Context, username, day, content string) error {
	if s.pool == nil {
		return ErrNoDB
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO day_plans (username, day, budget_minutes)
		 VALUES ($1, $2, 0)
		 ON CONFLICT (username, day) DO NOTHING`,
		username, day); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE day_plans SET report = $3, updated = now()
		 WHERE username = $1 AND day = $2`,
		username, day, content)
	return err
}
