-- +goose Up
-- Раздел «Чтение»: время чтения по дням. Секунды суммируются за календарный
-- день (дату считает клиент в своей таймзоне и присылает в запросе).
CREATE TABLE IF NOT EXISTS book_reading (
    username TEXT NOT NULL,
    date     DATE NOT NULL,
    seconds  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (username, date)
);

-- +goose Down
DROP TABLE IF EXISTS book_reading;
