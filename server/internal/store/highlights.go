package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BookHighlight — цветное выделение диапазона текста книги. Позиции Start/End
// — в символах от начала текста (как считает JS: единицы UTF-16).
type BookHighlight struct {
	ID      int    `json:"id"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Color   string `json:"color"`
	Excerpt string `json:"excerpt"`
	Created string `json:"created"`
}

// Единый формат времени создания выделения (RFC3339, UTC).
const bookHighlightCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// Highlights — хранилище выделений текста в книгах (таблица book_highlights).
type Highlights struct{ pool *pgxpool.Pool }

// NewHighlights создаёт хранилище выделений.
func NewHighlights(pool *pgxpool.Pool) *Highlights { return &Highlights{pool: pool} }

// BookOwned проверяет, что книга существует и принадлежит пользователю.
func (s *Highlights) BookOwned(ctx context.Context, username string, bookID int) bool {
	if s.pool == nil {
		return false
	}
	var one int
	err := s.pool.QueryRow(ctx,
		`SELECT 1 FROM books WHERE id = $1 AND username = $2`, bookID, username).Scan(&one)
	return err == nil
}

// List возвращает выделения книги в порядке по тексту.
func (s *Highlights) List(ctx context.Context, username string, bookID int) ([]BookHighlight, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, start_offset, end_offset, color, excerpt, `+bookHighlightCreatedExpr+`
		 FROM book_highlights
		 WHERE username = $1 AND book_id = $2
		 ORDER BY start_offset, id`,
		username, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BookHighlight, 0)
	for rows.Next() {
		var h BookHighlight
		if err := rows.Scan(&h.ID, &h.Start, &h.End, &h.Color, &h.Excerpt, &h.Created); err == nil {
			out = append(out, h)
		}
	}
	return out, rows.Err()
}

// Create сохраняет выделение диапазона [start, end) указанным цветом.
func (s *Highlights) Create(ctx context.Context, username string, bookID, start, end int, color, excerpt string) (BookHighlight, error) {
	if s.pool == nil {
		return BookHighlight{}, ErrNoDB
	}
	var h BookHighlight
	err := s.pool.QueryRow(ctx,
		`INSERT INTO book_highlights (username, book_id, start_offset, end_offset, color, excerpt)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, start_offset, end_offset, color, excerpt, `+bookHighlightCreatedExpr,
		username, bookID, start, end, color, excerpt).
		Scan(&h.ID, &h.Start, &h.End, &h.Color, &h.Excerpt, &h.Created)
	return h, err
}

// Delete удаляет выделение книги. ok=false — выделения не было.
func (s *Highlights) Delete(ctx context.Context, username string, bookID, highlightID int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM book_highlights WHERE id = $1 AND book_id = $2 AND username = $3`,
		highlightID, bookID, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
