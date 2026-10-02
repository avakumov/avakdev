-- +goose Up
-- Раздел «Чтение»: длина текста книги без разметки — знаменатель для процента
-- прочтения (числитель — позиция последней закладки).
-- Заполняем и для уже загруженных книг: тот же способ, что и при загрузке
-- новых, — вырезаем теги и берём число символов (картинки и стили лежат внутри
-- тегов, поэтому в длину текста не попадают).
ALTER TABLE books ADD COLUMN IF NOT EXISTS text_len INTEGER NOT NULL DEFAULT 0;

UPDATE books
SET text_len = length(regexp_replace(html, '<[^>]*>', '', 'g'))
WHERE text_len = 0 AND html <> '';

-- +goose Down
ALTER TABLE books DROP COLUMN IF EXISTS text_len;
