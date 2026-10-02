package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Note — конспект знаний по конкретной теме. Генерируется через ИИ,
// удачные варианты сохраняются для повторения.
type Note struct {
	// ID — уникальный идентификатор конспекта.
	ID int `json:"id"`
	// Topic — тема, по которой сгенерирован конспект.
	Topic string `json:"topic"`
	// Title — короткий заголовок (уникален, часто равен теме).
	Title string `json:"title"`
	// Content — текст конспекта (Markdown).
	Content string `json:"content"`
	// Repetitions — счётчик выполненных повторений конспекта.
	// Увеличивается на единицу, когда пользователь нажал кнопку «Я повторил».
	// При создании всегда равен 0.
	Repetitions int `json:"repetitions"`
	// Created — время создания (RFC3339, UTC).
	Created string `json:"created"`
	// Updated — время последнего изменения (RFC3339, UTC).
	Updated string `json:"updated"`
	// ReadingMinutes — время чтения конспекта в минутах, рассчитанное по
	// скорости чтения пользователя из профиля. Не хранится в БД:
	// вычисляется на лету в ListNotes и пересчитывается при изменении
	// reading_speed.
	ReadingMinutes int `json:"reading_minutes,omitempty"`
}

// audioData — сохранённое аудио для конспекта.
type audioData struct {
	Data []byte
	MIME string
}

// NoteStore — хранилище конспектов знаний.
// Если PostgreSQL настроена, заметки хранятся в таблице knowledge_notes
// (и кэшируются в памяти). Иначе используется in-memory мапа без персистентности.
type NoteStore struct {
	mu     sync.Mutex
	data   map[int]Note // кэш в памяти / хранилище без БД
	nextID int
	pool   *pgxpool.Pool
	// audio — кэш сгенерированных аудио: noteID -> (bytes, MIME).
	// Хранится в БД в таблице knowledge_notes_audio, в памяти — для быстрого доступа.
	audio map[int]audioData
}

// NewNoteStore создаёт хранилище конспектов и подгружает данные из БД
// (pool == nil — работаем без БД, только в памяти).
func NewNoteStore(pool *pgxpool.Pool) (*NoteStore, error) {
	ns := &NoteStore{
		data:   make(map[int]Note),
		nextID: 1,
		pool:   pool,
		audio:  make(map[int]audioData),
	}
	if pool == nil {
		return ns, nil
	}

	rows, err := pool.Query(context.Background(),
		`SELECT id,
		        topic,
		        title,
		        content,
		        repetitions,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM knowledge_notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maxID := 0
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Topic, &n.Title, &n.Content,
			&n.Repetitions, &n.Created, &n.Updated); err != nil {
			return nil, err
		}
		ns.data[n.ID] = n
		if n.ID > maxID {
			maxID = n.ID
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Следующий autoincrement не ниже уже занятых ID.
	ns.nextID = maxID + 1

	// Подгружаем аудио из БД в память.
	audioRows, err := pool.Query(context.Background(),
		`SELECT note_id, data, mime FROM knowledge_notes_audio`)
	if err != nil {
		return nil, err
	}
	defer audioRows.Close()
	for audioRows.Next() {
		var id int
		var ad audioData
		if err := audioRows.Scan(&id, &ad.Data, &ad.MIME); err != nil {
			return nil, err
		}
		ns.audio[id] = ad
	}
	if err := audioRows.Err(); err != nil {
		return nil, err
	}

	return ns, nil
}

// List возвращает все конспекты, отсортированные по дате обновления (новые сверху).
func (ns *NoteStore) List() []Note {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	out := make([]Note, 0, len(ns.data))
	for _, n := range ns.data {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out
}

// Get возвращает конспект по ID.
func (ns *NoteStore) Get(id int) (Note, bool) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	n, ok := ns.data[id]
	return n, ok
}

// Create добавляет новый конспект. Счётчик повторений всегда стартует с нуля
// и увеличивается только через MarkRepeat (кнопка «Я повторил»).
// При наличии БД пишет в таблицу, иначе — только в память.
func (ns *NoteStore) Create(topic, title, content string) (Note, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	n := Note{
		Topic:       topic,
		Title:       title,
		Content:     content,
		Repetitions: 0,
		Created:     now,
		Updated:     now,
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.pool != nil {
		err := ns.pool.QueryRow(context.Background(),
			`INSERT INTO knowledge_notes (topic, title, content, repetitions)
			 VALUES ($1, $2, $3, 0)
			 RETURNING id, to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			topic, title, content).
			Scan(&n.ID, &n.Created)
		if err != nil {
			return Note{}, err
		}
		if n.ID >= ns.nextID {
			ns.nextID = n.ID + 1
		}
	} else {
		n.ID = ns.nextID
		ns.nextID++
	}

	ns.data[n.ID] = n
	return n, nil
}

// Update обновляет существующий конспект (title, content).
// Счётчик повторений здесь не меняется — он управляется только через MarkRepeat.
func (ns *NoteStore) Update(id int, title, content string) (Note, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	n, ok := ns.data[id]
	if !ok {
		return Note{}, fmt.Errorf("конспект с ID %d не найден", id)
	}

	if title != "" {
		n.Title = title
	}
	if content != "" {
		n.Content = content
	}
	n.Updated = time.Now().UTC().Format(time.RFC3339)

	if ns.pool != nil {
		if _, err := ns.pool.Exec(context.Background(),
			`UPDATE knowledge_notes
			 SET title = $2, content = $3, updated = now()
			 WHERE id = $1`,
			id, n.Title, n.Content); err != nil {
			return Note{}, err
		}
	}

	ns.data[id] = n
	return n, nil
}

// MarkRepeat увеличивает счётчик повторений конспекта на единицу.
// Вызывается при нажатии пользователем кнопки «Я повторил».
func (ns *NoteStore) MarkRepeat(id int) (Note, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	n, ok := ns.data[id]
	if !ok {
		return Note{}, fmt.Errorf("конспект с ID %d не найден", id)
	}

	n.Repetitions++
	n.Updated = time.Now().UTC().Format(time.RFC3339)

	if ns.pool != nil {
		if _, err := ns.pool.Exec(context.Background(),
			`UPDATE knowledge_notes
			 SET repetitions = repetitions + 1, updated = now()
			 WHERE id = $1`, id); err != nil {
			return Note{}, err
		}
	}

	ns.data[id] = n
	return n, nil
}

// DeleteByID удаляет конспект по ID. Аудио удаляется каскадно из БД
// (ON DELETE CASCADE) и из памяти.
func (ns *NoteStore) DeleteByID(id int) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if _, ok := ns.data[id]; !ok {
		return fmt.Errorf("конспект с ID %d не найден", id)
	}

	if ns.pool != nil {
		if _, err := ns.pool.Exec(context.Background(),
			`DELETE FROM knowledge_notes WHERE id = $1`, id); err != nil {
			return err
		}
	}

	delete(ns.data, id)
	delete(ns.audio, id)
	return nil
}

// HasAudio возвращает true, если для конспекта с указанным ID уже
// сгенерировано и сохранено аудио.
func (ns *NoteStore) HasAudio(id int) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	_, ok := ns.audio[id]
	return ok
}

// GetAudio возвращает сохранённое аудио для конспекта и его MIME-тип.
func (ns *NoteStore) GetAudio(id int) ([]byte, string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ad, ok := ns.audio[id]
	if !ok {
		return nil, ""
	}
	return ad.Data, ad.MIME
}

// SaveAudio сохраняет аудио для конспекта. При наличии БД пишет в таблицу
// knowledge_notes_audio (UPSERT), также кэширует в памяти.
func (ns *NoteStore) SaveAudio(id int, data []byte, mime string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.pool != nil {
		if _, err := ns.pool.Exec(context.Background(),
			`INSERT INTO knowledge_notes_audio (note_id, data, mime)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (note_id) DO UPDATE SET data = $2, mime = $3`,
			id, data, mime); err != nil {
			return err
		}
	}

	ns.audio[id] = audioData{Data: data, MIME: mime}
	return nil
}
