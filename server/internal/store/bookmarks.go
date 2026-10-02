package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BookBookmark — закладка книги.
type BookBookmark struct {
	ID      int    `json:"id"`
	Anchor  int    `json:"anchor"`
	Excerpt string `json:"excerpt"`
	Created string `json:"created"`
}

// LastBookmark — последняя закладка пользователя по всем книгам.
type LastBookmark struct {
	BookID    int    `json:"book_id"`
	BookTitle string `json:"book_title"`
	Anchor    int    `json:"anchor"`
	Excerpt   string `json:"excerpt"`
}

// Единый формат времени создания закладки (RFC3339, UTC).
const bookBookmarkCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// Bookmarks — хранилище закладок книг (таблица book_bookmarks).
type Bookmarks struct{ pool *pgxpool.Pool }

// NewBookmarks создаёт хранилище закладок.
func NewBookmarks(pool *pgxpool.Pool) *Bookmarks { return &Bookmarks{pool: pool} }

// BookOwned проверяет, что книга существует и принадлежит пользователю.
func (s *Bookmarks) BookOwned(ctx context.Context, username string, bookID int) bool {
	if s.pool == nil {
		return false
	}
	var one int
	err := s.pool.QueryRow(ctx,
		`SELECT 1 FROM books WHERE id = $1 AND username = $2`, bookID, username).Scan(&one)
	return err == nil
}

// Last возвращает последнюю добавленную закладку пользователя (по всем книгам,
// кроме прочитанных). Если закладок нет — (nil, nil).
func (s *Bookmarks) Last(ctx context.Context, username string) (*LastBookmark, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	var out LastBookmark
	err := s.pool.QueryRow(ctx,
		`SELECT bm.book_id, b.title, bm.anchor, bm.excerpt
		 FROM book_bookmarks bm
		 JOIN books b ON b.id = bm.book_id AND b.username = bm.username
		 WHERE bm.username = $1 AND b.finished_at IS NULL
		 ORDER BY bm.created DESC, bm.id DESC
		 LIMIT 1`,
		username).Scan(&out.BookID, &out.BookTitle, &out.Anchor, &out.Excerpt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// List возвращает закладки книги в порядке по тексту.
func (s *Bookmarks) List(ctx context.Context, username string, bookID int) ([]BookBookmark, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, anchor, excerpt, `+bookBookmarkCreatedExpr+`
		 FROM book_bookmarks
		 WHERE username = $1 AND book_id = $2
		 ORDER BY anchor, id`,
		username, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BookBookmark, 0)
	for rows.Next() {
		var b BookBookmark
		if err := rows.Scan(&b.ID, &b.Anchor, &b.Excerpt, &b.Created); err == nil {
			out = append(out, b)
		}
	}
	return out, rows.Err()
}

// Create сохраняет закладку на выделенном фрагменте.
func (s *Bookmarks) Create(ctx context.Context, username string, bookID, anchor int, excerpt string) (BookBookmark, error) {
	if s.pool == nil {
		return BookBookmark{}, ErrNoDB
	}
	var b BookBookmark
	err := s.pool.QueryRow(ctx,
		`INSERT INTO book_bookmarks (username, book_id, anchor, excerpt)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, anchor, excerpt, `+bookBookmarkCreatedExpr,
		username, bookID, anchor, excerpt).
		Scan(&b.ID, &b.Anchor, &b.Excerpt, &b.Created)
	return b, err
}

// Delete удаляет закладку книги. ok=false — закладки не было.
func (s *Bookmarks) Delete(ctx context.Context, username string, bookID, bookmarkID int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM book_bookmarks WHERE id = $1 AND book_id = $2 AND username = $3`,
		bookmarkID, bookID, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
