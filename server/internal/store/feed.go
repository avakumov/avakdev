package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FeedItem — элемент ленты («вопрос-ответ»).
type FeedItem struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	// Topic — раздел (область) элемента: короткое слово вроде «golang».
	Topic    string `json:"topic"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	// Details — объяснение и примеры (Markdown, может быть пустым).
	Details string `json:"details"`
	Views   int    `json:"views"`
	// KnowCount/UnknownCount — сколько раз отмечено «знаю» / «не знаю».
	KnowCount    int    `json:"know_count"`
	UnknownCount int    `json:"unknown_count"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
}

// Единый формат времени элемента (RFC3339, UTC).
const (
	feedCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	feedUpdatedExpr = `to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	feedSelectCols  = `id, kind, topic, question, answer, details, views, know_count, unknown_count, ` +
		feedCreatedExpr + `, ` + feedUpdatedExpr
)

// feedScan собирает FeedItem из строки результата (порядок — как в feedSelectCols).
func feedScan(row interface{ Scan(...any) error }) (FeedItem, error) {
	var it FeedItem
	err := row.Scan(&it.ID, &it.Kind, &it.Topic, &it.Question, &it.Answer, &it.Details,
		&it.Views, &it.KnowCount, &it.UnknownCount, &it.Created, &it.Updated)
	return it, err
}

// Feed — хранилище элементов ленты (таблица feed_items).
type Feed struct{ pool *pgxpool.Pool }

// NewFeed создаёт хранилище ленты.
func NewFeed(pool *pgxpool.Pool) *Feed { return &Feed{pool: pool} }

// List возвращает элементы ленты пользователя (свежие сверху).
func (s *Feed) List(ctx context.Context, username string) ([]FeedItem, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+feedSelectCols+`
		 FROM feed_items WHERE username = $1
		 ORDER BY id DESC`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]FeedItem, 0)
	for rows.Next() {
		if it, err := feedScan(rows); err == nil {
			out = append(out, it)
		}
	}
	return out, rows.Err()
}

// Create добавляет элемент ленты.
func (s *Feed) Create(ctx context.Context, username, kind, topic, question, answer, details string) (FeedItem, error) {
	if s.pool == nil {
		return FeedItem{}, ErrNoDB
	}
	return feedScan(s.pool.QueryRow(ctx,
		`INSERT INTO feed_items (username, kind, topic, question, answer, details)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+feedSelectCols,
		username, kind, topic, question, answer, details))
}

// Update меняет раздел, вопрос, ответ и объяснение (показы не трогаем).
func (s *Feed) Update(ctx context.Context, username string, id int, kind, topic, question, answer, details string) (FeedItem, error) {
	if s.pool == nil {
		return FeedItem{}, ErrNoDB
	}
	return feedScan(s.pool.QueryRow(ctx,
		`UPDATE feed_items
		 SET kind = $3, topic = $4, question = $5, answer = $6, details = $7, updated = now()
		 WHERE id = $1 AND username = $2
		 RETURNING `+feedSelectCols,
		id, username, kind, topic, question, answer, details))
}

// Delete удаляет элемент ленты. ok=false — элемента не было.
func (s *Feed) Delete(ctx context.Context, username string, id int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM feed_items WHERE id = $1 AND username = $2`, id, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// BulkCreate сохраняет сразу несколько элементов ленты одним запросом.
func (s *Feed) BulkCreate(ctx context.Context, username, kind string,
	topics, questions, answers, details []string) ([]FeedItem, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`INSERT INTO feed_items (username, kind, topic, question, answer, details)
		 SELECT $1, $2, t, q, a, d
		 FROM unnest($3::text[], $4::text[], $5::text[], $6::text[]) AS x(t, q, a, d)
		 RETURNING `+feedSelectCols,
		username, kind, topics, questions, answers, details)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]FeedItem, 0, len(topics))
	for rows.Next() {
		if it, err := feedScan(rows); err == nil {
			out = append(out, it)
		}
	}
	return out, rows.Err()
}

// React увеличивает счётчик «знаю» (know=true) или «не знаю» (know=false).
func (s *Feed) React(ctx context.Context, username string, id int, know bool) (FeedItem, error) {
	if s.pool == nil {
		return FeedItem{}, ErrNoDB
	}
	// Две готовые ветки вместо подстановки имени колонки: без динамического SQL.
	q := `UPDATE feed_items SET unknown_count = unknown_count + 1
		WHERE id = $1 AND username = $2
		RETURNING ` + feedSelectCols
	if know {
		q = `UPDATE feed_items SET know_count = know_count + 1
		WHERE id = $1 AND username = $2
		RETURNING ` + feedSelectCols
	}
	return feedScan(s.pool.QueryRow(ctx, q, id, username))
}

// View отмечает показ элемента в ленте: views = views + 1.
func (s *Feed) View(ctx context.Context, username string, id int) (FeedItem, error) {
	if s.pool == nil {
		return FeedItem{}, ErrNoDB
	}
	return feedScan(s.pool.QueryRow(ctx,
		`UPDATE feed_items SET views = views + 1
		 WHERE id = $1 AND username = $2
		 RETURNING `+feedSelectCols,
		id, username))
}
