package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"avakumov/server/internal/httpkit"
)

// «Входящие» колокольчика: уведомления канала app, наступившие по расписанию
// и ещё не закрытые пользователем. SQL живёт в store.Notifications.

// deliverDueAppNotifications кладёт во «входящие» наступившие уведомления
// канала app (по одному на уведомление, пока оно не закрыто).
func deliverDueAppNotifications() {
	if err := notificationsStore.DeliverDueApp(context.Background()); err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: входящие (app): %v", err)
	}
}

// handleListNotificationInbox отдаёт «входящие» (наступившие и не закрытые).
func handleListNotificationInbox(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(session)

	out, err := notificationsStore.Inbox(context.Background(), sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить уведомления"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"inbox": out})
}

// handleDismissNotification закрывает «входящее» уведомление. Если это было
// одноразовое — оно удаляется; периодическое — сдвигается на следующий период.
func handleDismissNotification(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	notifID, err := notificationsStore.DismissInbox(context.Background(), sessData.username, id)
	if err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Уведомление не найдено"})
		return
	}

	// Действие с родительским уведомлением.
	ntype, dueAt, unit, value, err := notificationsStore.Meta(context.Background(), sessData.username, notifID)
	if err != nil {
		// Родитель уже удалён — входящее закрыто, и этого достаточно.
		c.JSON(http.StatusOK, httpkit.H{"ok": true})
		return
	}

	if ntype == notifOnce {
		if err := notificationsStore.DeleteByID(context.Background(), notifID); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ: удаление одноразового #%d: %v", notifID, err)
		}
		notifications.removeFromMemory(notifID)
	} else if ntype == notifPeriodic {
		due, err := time.Parse(time.RFC3339, dueAt)
		if err == nil {
			next := advanceDue(due.UTC(), unit, value).Format(time.RFC3339)
			if err := notificationsStore.SetDue(context.Background(), notifID, next); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ: сдвиг периодического #%d: %v", notifID, err)
			}
			notifications.patchDue(notifID, next)
		}
	}

	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
