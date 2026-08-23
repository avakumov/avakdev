package main

import (
	"strings"
	"testing"
)

// Тест хранилища задач по модификации приложения (in-memory, без БД):
// создание, валидация, смена статуса, приватность между пользователями.
func TestAppTaskStore(t *testing.T) {
	appTasks = &appTaskStore{
		data:   make(map[int]AppTask),
		nextID: 1,
		hasDB:  false,
	}

	// Создание.
	task, err := appTasks.create("admin", "Исправить отступ", "В карточке метрики слишком большой отступ")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Status != taskStatusNew {
		t.Fatalf("новая задача должна иметь статус new, got %q", task.Status)
	}

	// Пустой заголовок отклоняется.
	if _, err := appTasks.create("admin", "   ", "описание"); err == nil {
		t.Fatal("create с пустым заголовком должен падать")
	}

	// Смена статуса.
	updated, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusInProgress, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != taskStatusInProgress {
		t.Fatalf("status = %q, want %q", updated.Status, taskStatusInProgress)
	}

	// Некорректный статус отклоняется.
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, "banana", nil, nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("update с неизвестным статусом должен падать")
	}

	// Результат, журнал, хэш коммита записываются только когда переданы.
	res := "изменён отступ в Metrics.jsx"
	logText := "[1] read_file(...)\nфайл записан"
	hash := "abc123def456"
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusDone, &res, &logText, nil, nil, &hash, nil, nil); err != nil {
		t.Fatalf("update с результатом: %v", err)
	}
	if got, _ := appTasks.getOwned("admin", task.ID); got.Result != res || got.Log != logText || got.Status != taskStatusDone || got.CommitHash != hash {
		t.Fatalf("result/log/status/hash не сохранились: %+v", got)
	}
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusInProgress, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("update без результата: %v", err)
	}
	if got, _ := appTasks.getOwned("admin", task.ID); got.Result != res || got.Log != logText || got.CommitHash != hash {
		t.Fatalf("update с nil затёр result/log/hash: %q / %q / %q", got.Result, got.Log, got.CommitHash)
	}

	// Флаги деплоя и отката записываются только когда переданы.
	req := true
	deployedAt := "2026-08-23T12:00:00Z"
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusDone, nil, nil, &req, &deployedAt, nil, &req, nil); err != nil {
		t.Fatalf("update с флагами: %v", err)
	}
	if got, _ := appTasks.getOwned("admin", task.ID); !got.DeployRequested || got.DeployedAt != deployedAt || !got.RevertRequested {
		t.Fatalf("флаги деплоя/отката не сохранились: %+v", got)
	}

	// Приватность: другой пользователь не видит и не трогает чужие задачи.
	if tasks := appTasks.list("other"); len(tasks) != 0 {
		t.Fatalf("list(other) = %d задач, want 0", len(tasks))
	}
	if _, err := appTasks.update("other", task.ID, "Чужая", "", taskStatusDone, nil, nil, nil, nil, nil, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "не найдена") {
		t.Fatalf("update чужой задачи должен падать: %v", err)
	}
	if err := appTasks.delete("other", task.ID); err == nil {
		t.Fatal("delete чужой задачи должен падать")
	}

	// Удаление.
	if err := appTasks.delete("admin", task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := appTasks.getOwned("admin", task.ID); ok {
		t.Fatal("задача должна исчезнуть после delete")
	}
}
