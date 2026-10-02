package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// ListNotificationInbox отдаёт «входящие» (наступившие и не закрытые).
func (h *Handlers) ListNotificationInbox(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)

	out, err := h.App.NotifDB.Inbox(context.Background(), sessData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось загрузить уведомления"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"inbox": out})
}

// DismissNotification закрывает «входящее» уведомление. Если это было
// одноразовое — оно удаляется; периодическое — сдвигается на следующий период.
func (h *Handlers) DismissNotification(c *httpkit.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)

	notifID, err := h.App.NotifDB.DismissInbox(context.Background(), sessData.Username, id)
	if err != nil {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Уведомление не найдено"})
		return
	}

	// Действие с родительским уведомлением.
	ntype, dueAt, unit, value, err := h.App.NotifDB.Meta(context.Background(), sessData.Username, notifID)
	if err != nil {
		// Родитель уже удалён — входящее закрыто, и этого достаточно.
		c.JSON(http.StatusOK, httpkit.H{"ok": true})
		return
	}

	if ntype == app.NotifOnce {
		if err := h.App.NotifDB.DeleteByID(context.Background(), notifID); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ: удаление одноразового #%d: %v", notifID, err)
		}
		h.App.Notifications.RemoveFromMemory(notifID)
	} else if ntype == app.NotifPeriodic {
		due, err := time.Parse(time.RFC3339, dueAt)
		if err == nil {
			next := app.AdvanceDue(due.UTC(), unit, value).Format(time.RFC3339)
			if err := h.App.NotifDB.SetDue(context.Background(), notifID, next); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ: сдвиг периодического #%d: %v", notifID, err)
			}
			h.App.Notifications.PatchDue(notifID, next)
		}
	}

	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
