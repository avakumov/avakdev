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
	// Pages — число страниц PDF (у fb2/epub 0).
	Pages int `json:"pages,omitempty"`
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
		        b.pages,
		        CASE WHEN b.format = 'pdf' THEN b.pages ELSE b.text_len END,
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
		var denom, lastAnchor int
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.Created, &b.FinishedAt,
			&b.StartedAt, &b.Pages, &denom, &lastAnchor); err != nil {
			continue
		}
		b.ReadPercent = bookReadPercent(denom, lastAnchor, b.FinishedAt != "")
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
		`SELECT id, title, author, format, html, pages, `+bookCreatedExpr+`, `+bookFinishedExpr+`
		 FROM books WHERE id = $1 AND username = $2`,
		id, username).
		Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.HTML, &b.Pages, &b.Created, &b.FinishedAt)
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

// CreateFile сохраняет книгу-файл (PDF) как есть: строку books и байты в
// book_files — в одной транзакции.
func (s *Books) CreateFile(ctx context.Context, username, title, author, format, mime string, data []byte) (Book, error) {
	if s.pool == nil {
		return Book{}, ErrNoDB
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback(ctx)

	b := Book{Title: title, Author: author, Format: format}
	if err := tx.QueryRow(ctx,
		`INSERT INTO books (username, title, author, format, html, text_len)
		 VALUES ($1, $2, $3, $4, '', 0)
		 RETURNING id, `+bookCreatedExpr,
		username, title, author, format).Scan(&b.ID, &b.Created); err != nil {
		return Book{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO book_files (book_id, data, mime) VALUES ($1, $2, $3)`,
		b.ID, data, mime); err != nil {
		return Book{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Book{}, err
	}
	return b, nil
}

// SetPages сохраняет число страниц PDF (сообщает клиент). Мусорные значения
// (<= 0) игнорируем. ok=false — книги нет.
func (s *Books) SetPages(ctx context.Context, username string, id, pages int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	if pages <= 0 {
		return false, nil
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE books SET pages = $3 WHERE id = $1 AND username = $2`,
		id, username, pages)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// File возвращает байты PDF и MIME книги. ok=false — книги/файла нет.
func (s *Books) File(ctx context.Context, username string, id int) (data []byte, mime string, ok bool) {
	if s.pool == nil {
		return nil, "", false
	}
	err := s.pool.QueryRow(ctx,
		`SELECT f.data, f.mime
		   FROM book_files f
		   JOIN books b ON b.id = f.book_id
		  WHERE b.id = $1 AND b.username = $2`,
		id, username).Scan(&data, &mime)
	if err != nil {
		return nil, "", false
	}
	return data, mime, true
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
