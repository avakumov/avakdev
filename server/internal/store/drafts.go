package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DraftNote — заметка-черновик пользователя.
type DraftNote struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// Единый формат времени заметки (RFC3339, UTC).
const (
	draftCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	draftUpdatedExpr = `to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
)

// Drafts — хранилище заметок (таблица draft_notes).
type Drafts struct{ pool *pgxpool.Pool }

// NewDrafts создаёт хранилище заметок.
func NewDrafts(pool *pgxpool.Pool) *Drafts { return &Drafts{pool: pool} }

// List возвращает заметки пользователя: свежие сверху.
func (s *Drafts) List(ctx context.Context, username string) ([]DraftNote, error) {
	if s.pool == nil {
		return nil, ErrNoDB
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr+`
		 FROM draft_notes WHERE username = $1
		 ORDER BY updated DESC, id DESC`,
		username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DraftNote, 0)
	for rows.Next() {
		var n DraftNote
		if err := rows.Scan(&n.ID, &n.Content, &n.Created, &n.Updated); err == nil {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}

// Create сохраняет новую заметку.
func (s *Drafts) Create(ctx context.Context, username, content string) (DraftNote, error) {
	if s.pool == nil {
		return DraftNote{}, ErrNoDB
	}
	var n DraftNote
	err := s.pool.QueryRow(ctx,
		`INSERT INTO draft_notes (username, content)
		 VALUES ($1, $2)
		 RETURNING id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr,
		username, content).
		Scan(&n.ID, &n.Content, &n.Created, &n.Updated)
	return n, err
}

// Update заменяет текст существующей заметки пользователя.
func (s *Drafts) Update(ctx context.Context, username string, id int, content string) (DraftNote, error) {
	if s.pool == nil {
		return DraftNote{}, ErrNoDB
	}
	var n DraftNote
	err := s.pool.QueryRow(ctx,
		`UPDATE draft_notes SET content = $3, updated = now()
		 WHERE id = $1 AND username = $2
		 RETURNING id, content, `+draftCreatedExpr+`, `+draftUpdatedExpr,
		id, username, content).
		Scan(&n.ID, &n.Content, &n.Created, &n.Updated)
	return n, err
}

// Delete удаляет заметку пользователя. ok=false — заметки не было.
func (s *Drafts) Delete(ctx context.Context, username string, id int) (bool, error) {
	if s.pool == nil {
		return false, ErrNoDB
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM draft_notes WHERE id = $1 AND username = $2`, id, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
