package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Profile — пользовательский профиль: описание для генерации резюме
// и сгенерированное резюме.
type Profile struct {
	// Description — короткое описание работника/соискателя,
	// на основе которого генерируется резюме.
	Description string `json:"description"`
	// Resume — сгенерированное DeepSeek резюме (в HTML/текстовом виде).
	// Содержит плейсхолдер <img id="resume-photo">, в который на сервере
	// подставляется актуальное фото.
	Resume string `json:"resume"`
	// PhotoMime — MIME-тип прикреплённого фото (например image/jpeg).
	PhotoMime string `json:"photo_mime"`
	// PhotoData — прикреплённое фото в base64 (без data URI префикса).
	PhotoData string `json:"photo_data"`
	// Updated — время последнего изменения профиля (RFC3339, UTC).
	Updated string `json:"updated"`
}

// ProfileStore — хранилище профиля.
// Если база данных PostgreSQL настроена, профиль хранится в таблице profile
// (единственная строка id=1). Иначе используется in-memory структура.
type ProfileStore struct {
	mu   sync.Mutex
	data Profile
	pool *pgxpool.Pool
}

// NewProfileStore создаёт хранилище профиля и подгружает данные из БД
// (pool == nil — работаем без БД).
func NewProfileStore(pool *pgxpool.Pool) (*ProfileStore, error) {
	ps := &ProfileStore{pool: pool}
	if pool == nil {
		return ps, nil
	}

	var description, resume, photo, photoMime, updated string
	err := pool.QueryRow(context.Background(),
		`SELECT description,
		        resume,
		        photo,
		        photo_mime,
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM profile WHERE id = 1`).
		Scan(&description, &resume, &photo, &photoMime, &updated)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	ps.data = Profile{
		Description: description,
		Resume:      resume,
		PhotoMime:   photoMime,
		PhotoData:   photo,
		Updated:     updated,
	}
	return ps, nil
}

// Get возвращает текущий профиль.
func (ps *ProfileStore) Get() Profile {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.data
}

// SaveDescription сохраняет только описание (резюме не трогаем).
func (ps *ProfileStore) SaveDescription(description string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: description,
		Resume:      ps.data.Resume,
		PhotoMime:   ps.data.PhotoMime,
		PhotoData:   ps.data.PhotoData,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.pool != nil {
		if _, err := ps.pool.Exec(context.Background(),
			`UPDATE profile SET description = $1, updated = now() WHERE id = 1`,
			description); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// SaveResume сохраняет сгенерированное резюме (описание не трогаем).
func (ps *ProfileStore) SaveResume(resume string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: ps.data.Description,
		Resume:      resume,
		PhotoMime:   ps.data.PhotoMime,
		PhotoData:   ps.data.PhotoData,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.pool != nil {
		if _, err := ps.pool.Exec(context.Background(),
			`UPDATE profile SET resume = $1, updated = now() WHERE id = 1`,
			resume); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// SavePhoto сохраняет фото (base64 + MIME) в профиль.
func (ps *ProfileStore) SavePhoto(data, mime string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: ps.data.Description,
		Resume:      ps.data.Resume,
		PhotoMime:   mime,
		PhotoData:   data,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.pool != nil {
		if _, err := ps.pool.Exec(context.Background(),
			`UPDATE profile SET photo = $1, photo_mime = $2, updated = now() WHERE id = 1`,
			data, mime); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// ClearPhoto удаляет фото из профиля.
func (ps *ProfileStore) ClearPhoto() (Profile, error) {
	return ps.SavePhoto("", "")
}
