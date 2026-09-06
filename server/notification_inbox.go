package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// NotificationInboxItem — запись «входящих»: уведомление, наступившее по
// расписанию и ещё не закрытое пользователем (колокольчик).
type NotificationInboxItem struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Created string `json:"created"`
}

// deliverDueAppNotifications кладёт во «входящие» наступившие уведомления
// канала app (по одному на уведомление, пока оно не закрыто).
func deliverDueAppNotifications() {
	_, err := db.Exec(context.Background(),
		`INSERT INTO notification_inbox (username, notif_id, text)
		 SELECT n.username, n.id, n.text
		 FROM user_notifications n
		 WHERE n.channel = 'app'
		   AND n.due_at <= now()
		   AND NOT EXISTS (SELECT 1 FROM notification_inbox i WHERE i.notif_id = n.id)`)
	if err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: входящие (app): %v", err)
	}
}

// handleListNotificationInbox отдаёт «входящие» (наступившие и не закрытые).
func handleListNotificationInbox(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        text,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notification_inbox
		 WHERE username = $1
		 ORDER BY created DESC`,
		sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить уведомления"})
		return
	}
	defer rows.Close()

	out := make([]NotificationInboxItem, 0)
	for rows.Next() {
		var it NotificationInboxItem
		if err := rows.Scan(&it.ID, &it.Text, &it.Created); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить уведомления"})
			return
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить уведомления"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"inbox": out})
}

// handleDismissNotification закрывает «входящее» уведомление. Если это было
// одноразовое — оно удаляется; периодическое — сдвигается на следующий период.
func handleDismissNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var notifID int
	err = db.QueryRow(context.Background(),
		`DELETE FROM notification_inbox WHERE id = $1 AND username = $2
		 RETURNING notif_id`,
		id, sessData.username).Scan(&notifID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Уведомление не найдено"})
		return
	}

	// Действие с родительским уведомлением.
	var ntype, dueAt, unit string
	var value int
	err = db.QueryRow(context.Background(),
		`SELECT type,
		        to_char(due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        period_unit, period_value
		 FROM user_notifications WHERE id = $1 AND username = $2`,
		notifID, sessData.username).Scan(&ntype, &dueAt, &unit, &value)
	if err != nil {
		// Родитель уже удалён — входящее закрыто, и этого достаточно.
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if ntype == notifOnce {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM user_notifications WHERE id = $1`, notifID); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ: удаление одноразового #%d: %v", notifID, err)
		}
		notifications.removeFromMemory(notifID)
	} else if ntype == notifPeriodic {
		due, err := time.Parse(time.RFC3339, dueAt)
		if err == nil {
			next := advanceDue(due.UTC(), unit, value).Format(time.RFC3339)
			if _, err := db.Exec(context.Background(),
				`UPDATE user_notifications SET due_at = $2, updated = now()
				 WHERE id = $1`,
				notifID, next); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ: сдвиг периодического #%d: %v", notifID, err)
			}
			notifications.patchDue(notifID, next)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
