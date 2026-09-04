-- +goose Up
-- «Входящие» уведомления: сюда планировщик кладёт наступившие уведомления
-- канала app — колокольчик показывает их по мере наступления.
CREATE TABLE IF NOT EXISTS notification_inbox (
	id       SERIAL PRIMARY KEY,
	username TEXT NOT NULL,
	notif_id INT  NOT NULL,
	text     TEXT NOT NULL,
	created  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notification_inbox_username
	ON notification_inbox (username, created DESC);

-- +goose Down
DROP TABLE IF EXISTS notification_inbox;
