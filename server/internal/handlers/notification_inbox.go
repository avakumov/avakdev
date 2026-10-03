package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"avakumov/server/internal/app"
)

// ListNotificationInbox отдаёт «входящие» (наступившие и не закрытые).
func (h *Handlers) ListNotificationInbox(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)

	out, err := h.App.NotifDB.Inbox(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить уведомления"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inbox": out})
}

// DismissNotification закрывает «входящее» уведомление. Если это было
// одноразовое — оно удаляется; периодическое — сдвигается на следующий период.
func (h *Handlers) DismissNotification(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := sessionOf(r)

	notifID, err := h.App.NotifDB.DismissInbox(context.Background(), sessData.Username, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Уведомление не найдено"})
		return
	}

	// Действие с родительским уведомлением.
	ntype, dueAt, unit, value, err := h.App.NotifDB.Meta(context.Background(), sessData.Username, notifID)
	if err != nil {
		// Родитель уже удалён — входящее закрыто, и этого достаточно.
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
