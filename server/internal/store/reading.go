// Package store — доступ к данным в PostgreSQL. Здесь сосредоточены SQL-запросы
// разделов: хендлеры в main вызывают методы и SQL сами не пишут. Так проще
// тестировать логику данных и менять схему в одном месте.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoDB — база данных не настроена (пул не создан).
var ErrNoDB = errors.New("база данных не настроена")

// ReadingTime — время чтения за день и цель этого дня.
type ReadingTime struct {
	Date        string `json:"date"`
	Seconds     int    `json:"seconds"`
	GoalSeconds int    `json:"goal_seconds"`
}

// Reading — хранилище учёта времени чтения (таблица book_reading):
// секунды и цель дня, по одному значению на пользователя и дату.
type Reading struct{ pool *pgxpool.Pool }

// NewReading создаёт хранилище времени чтения.
func NewReading(pool *pgxpool.Pool) *Reading { return &Reading{pool: pool} }

// Day возвращает время и цель чтения за день. Записей за день нет — вернёт
// нули и цель по умолчанию defaultGoal.
func (s *Reading) Day(ctx context.Context, username, date string, defaultGoal int) (ReadingTime, error) {
	if s.pool == nil {
		return ReadingTime{}, ErrNoDB
	}
	out := ReadingTime{Date: date, GoalSeconds: defaultGoal}
	err := s.pool.QueryRow(ctx,
		`SELECT seconds, goal_seconds FROM book_reading WHERE username = $1 AND date = $2`,
		username, date).Scan(&out.Seconds, &out.GoalSeconds)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ReadingTime{}, err
	}
	return out, nil
}

// Add прибавляет секунды чтения к дню и возвращает новую сумму за день.
func (s *Reading) Add(ctx context.Context, username, date string, seconds int) (ReadingTime, error) {
	if s.pool == nil {
		return ReadingTime{}, ErrNoDB
	}
	out := ReadingTime{Date: date}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO book_reading (username, date, seconds)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (username, date)
		 DO UPDATE SET seconds = book_reading.seconds + EXCLUDED.seconds
		 RETURNING seconds, goal_seconds`,
		username, date, seconds).Scan(&out.Seconds, &out.GoalSeconds)
	if err != nil {
		return ReadingTime{}, err
	}
	return out, nil
}

// SetGoal задаёт цель чтения на день (строка дня создаётся при необходимости).
func (s *Reading) SetGoal(ctx context.Context, username, date string, goal int) (ReadingTime, error) {
	if s.pool == nil {
		return ReadingTime{}, ErrNoDB
	}
	out := ReadingTime{Date: date}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO book_reading (username, date, seconds, goal_seconds)
		 VALUES ($1, $2, 0, $3)
		 ON CONFLICT (username, date)
		 DO UPDATE SET goal_seconds = EXCLUDED.goal_seconds
		 RETURNING seconds, goal_seconds`,
		username, date, goal).Scan(&out.Seconds, &out.GoalSeconds)
	if err != nil {
		return ReadingTime{}, err
	}
	return out, nil
}

// History возвращает дни, в которые было чтение (сначала новые), не более limit.
// Дни с нулём секунд (например, только изменённая цель) не попадают.
func (s *Reading) History(ctx context.Context, username string, limit int) ([]ReadingTime, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT to_char(br.date,'YYYY-MM-DD'), br.seconds, br.goal_seconds
		 FROM book_reading br
		 WHERE br.username = $1 AND br.seconds > 0
		 ORDER BY br.date DESC
		 LIMIT $2`,
		username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ReadingTime, 0)
	for rows.Next() {
		var d ReadingTime
		if err := rows.Scan(&d.Date, &d.Seconds, &d.GoalSeconds); err == nil {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}
