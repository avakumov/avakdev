-- +goose Up
-- Пользовательские метрики: определения (тип: целое/дробное/да-нет)
-- и значения — одно на (метрика, день).
CREATE TABLE IF NOT EXISTS metric_definitions (
	id       SERIAL PRIMARY KEY,
	username TEXT NOT NULL,
	name     TEXT NOT NULL,
	type     TEXT NOT NULL CHECK (type IN ('int', 'float', 'bool')),
	created  TIMESTAMP NOT NULL DEFAULT now(),
	UNIQUE (username, name)
);

CREATE TABLE IF NOT EXISTS metric_values (
	metric_id INTEGER NOT NULL REFERENCES metric_definitions(id) ON DELETE CASCADE,
	date      DATE NOT NULL,
	value     TEXT NOT NULL,
	updated   TIMESTAMP NOT NULL DEFAULT now(),
	PRIMARY KEY (metric_id, date)
);

-- +goose Down
DROP TABLE IF EXISTS metric_values;
DROP TABLE IF EXISTS metric_definitions;
