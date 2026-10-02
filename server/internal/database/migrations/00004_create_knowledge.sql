-- +goose Up
CREATE TABLE IF NOT EXISTS knowledge_notes (
	id          SERIAL PRIMARY KEY,
	topic       TEXT NOT NULL,
	title       TEXT NOT NULL,
	content     TEXT NOT NULL DEFAULT '',
	repetitions INTEGER NOT NULL DEFAULT 1,
	created     TIMESTAMP NOT NULL DEFAULT now(),
	updated     TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge_notes_audio (
	note_id INTEGER PRIMARY KEY REFERENCES knowledge_notes(id) ON DELETE CASCADE,
	data    BYTEA NOT NULL,
	mime    TEXT NOT NULL DEFAULT 'audio/ogg',
	created TIMESTAMP NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS knowledge_notes_audio;
DROP TABLE IF EXISTS knowledge_notes;
