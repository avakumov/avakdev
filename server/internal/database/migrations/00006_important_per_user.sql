-- +goose Up
-- «Важное» сообщение становится персональным: у каждого пользователя своё.
-- Миграция выполняется в транзакции, поэтому переезд атомарен.
ALTER TABLE important_message RENAME TO important_message_old;

CREATE TABLE important_message (
	username   TEXT PRIMARY KEY,
	content    TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Старое общее сообщение копируем каждому пользователю как стартовое,
-- чтобы контент не потерялся. У новых пользователей строки не будет —
-- она создастся при первом сохранении своего сообщения.
INSERT INTO important_message (username, content, updated_by, updated_at)
SELECT u.username, old.content, old.updated_by, old.updated_at
FROM important_message_old old
CROSS JOIN users u;

DROP TABLE important_message_old;

-- +goose Down
-- Возвращаем одно общее сообщение: берём самое свежее из персональных.
ALTER TABLE important_message RENAME TO important_message_old;

CREATE TABLE important_message (
	id         INTEGER PRIMARY KEY,
	content    TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO important_message (id, content, updated_by, updated_at)
SELECT 1, content, updated_by, updated_at
FROM important_message_old
ORDER BY updated_at DESC NULLS LAST
LIMIT 1;

DROP TABLE important_message_old;
