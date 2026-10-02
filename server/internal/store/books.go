package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Book — книга пользователя (HTML-версия текста).
type Book struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Format  string `json:"format"`
	Created string `json:"created"`
	// FinishedAt — когда книга отмечена прочитанной (пусто — не прочитана).
	FinishedAt string `json:"finished_at"`
	// StartedAt — начало чтения: время первой закладки (пусто — закладок нет).
	StartedAt string `json:"started_at,omitempty"`
	// ReadPercent — сколько книги прочитано (0–100) по последней закладке.
	ReadPercent int `json:"read_percent,omitempty"`
	// HTML — сконвертированный текст; в списке не отдаётся (omitempty).
	HTML string `json:"html,omitempty"`
}

// Единый формат времени создания/прочтения (RFC3339, UTC).
const (
	bookCreatedExpr  = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	bookFinishedExpr = `COALESCE(to_char(finished_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')`
	bookStartedExpr  = `COALESCE(to_char((SELECT MIN(bm.created) FROM book_bookmarks bm
				WHERE bm.book_id = b.id AND bm.username = b.username) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')`
)

// bookReadPercent — процент прочтения книги: позиция последней закладки
// относительно длины текста. Отмеченная прочитанной книга — 100%.
func bookReadPercent(textLen, anchor int, finished bool) int {
	if finished {
		return 100
	}
	if textLen <= 0 || anchor <= 0 {
		return 0
	}
	percent := (anchor*100 + textLen/2) / textLen // с округлением
	if percent > 100 {
		return 100
	}
	return percent
}

// Books — хранилище книг (таблица books).
type Books struct{ pool *pgxpool.Pool }

// NewBooks создаёт хранилище книг.
func NewBooks(pool *pgxpool.Pool) *Books { return &Books{pool: pool} }

// List возвращает книги пользователя (без текста). Непрочитанные идут первыми,
// прочитанные — в конце. У каждой книги считается процент прочтения.
func (s *Books) List(ctx context.Context, username string) ([]Book, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT b.id, b.title, b.author, b.format, `+bookCreatedExpr+`, `+bookFinishedExpr+`, `+bookStartedExpr+`,
		        b.text_len,
		        COALESCE((SELECT MAX(bm.anchor) FROM book_bookmarks bm
		                  WHERE bm.book_id = b.id AND bm.username = b.username), 0)
		 FROM books b
		 WHERE b.username = $1
		 ORDER BY (b.finished_at IS NOT NULL), b.id DESC`,
		username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Book, 0)
	for rows.Next() {
		var b Book
		var textLen, lastAnchor int
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.Created, &b.FinishedAt,
			&b.StartedAt, &textLen, &lastAnchor); err != nil {
			continue
		}
		b.ReadPercent = bookReadPercent(textLen, lastAnchor, b.FinishedAt != "")
		out = append(out, b)
	}
	return out, rows.Err()
}

// Get возвращает книгу вместе с HTML-текстом. ok=false — книги нет.
func (s *Books) Get(ctx context.Context, username string, id int) (Book, bool) {
	if s.pool == nil {
		return Book{}, false
	}
	var b Book
	err := s.pool.QueryRow(ctx,
		`SELECT id, title, author, format, html, `+bookCreatedExpr+`, `+bookFinishedExpr+`
		 FROM books WHERE id = $1 AND username = $2`,
		id, username).
		Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.HTML, &b.Created, &b.FinishedAt)
	if err != nil {
		return Book{}, false
	}
	return b, true
}

// Create сохраняет книгу (text_len считается из HTML).
func (s *Books) Create(ctx context.Context, username, title, author, format, html string) (Book, error) {
	if s.pool == nil {
		return Book{}, ErrNoDB
	}
	b := Book{Title: title, Author: author, Format: format}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO books (username, title, author, format, html, text_len)
		 VALUES ($1, $2, $3, $4, $5, length(regexp_replace($5, '<[^>]*>', '', 'g')))
		 RETURNING id, `+bookCreatedExpr,
		username, title, author, format, html).
		Scan(&b.ID, &b.Created)
	return b, err
}

// Delete удаляет книгу. ok=false — книги не было.
func (s *Books) Delete(ctx context.Context, username string, id int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM books WHERE id = $1 AND username = $2`, id, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SetFinished отмечает книгу прочитанной (finished=true) или возвращает в
// чтение (false). Возвращает отметку времени; ok=false — книги нет.
func (s *Books) SetFinished(ctx context.Context, username string, id int, finished bool) (string, bool) {
	if s.pool == nil {
		return "", false
	}
	var finishedAt string
	err := s.pool.QueryRow(ctx,
		`UPDATE books
		 SET finished_at = CASE WHEN $3 THEN now() ELSE NULL END
		 WHERE id = $1 AND username = $2
		 RETURNING `+bookFinishedExpr,
		id, username, finished).Scan(&finishedAt)
	if err != nil {
		return "", false
	}
	return finishedAt, true
}
