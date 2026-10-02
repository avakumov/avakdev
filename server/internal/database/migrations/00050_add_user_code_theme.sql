-- +goose Up
-- Тема оформления блоков кода в профиле пользователя. Значение — id семейства
-- темы (night-owl, github, solarized, one, plain, dracula, monokai); светлый или
-- тёмный вариант подставляется по теме сайта. По умолчанию — night-owl.
ALTER TABLE users ADD COLUMN IF NOT EXISTS code_theme TEXT NOT NULL DEFAULT 'night-owl';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS code_theme;
