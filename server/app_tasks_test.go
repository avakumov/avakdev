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
	updated, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusInProgress, nil, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != taskStatusInProgress {
		t.Fatalf("status = %q, want %q", updated.Status, taskStatusInProgress)
	}

	// Некорректный статус отклоняется.
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, "banana", nil, nil); err == nil {
		t.Fatal("update с неизвестным статусом должен падать")
	}

	// Результат и журнал записываются только когда переданы (nil — не затирает).
	res := "изменён отступ в Metrics.jsx"
	logText := "[1] read_file(...)\nфайл записан"
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusDone, &res, &logText); err != nil {
		t.Fatalf("update с результатом: %v", err)
	}
	if got, _ := appTasks.getOwned("admin", task.ID); got.Result != res || got.Log != logText || got.Status != taskStatusDone {
		t.Fatalf("result/log/status не сохранились: %+v", got)
	}
	if _, err := appTasks.update("admin", task.ID, task.Title, task.Description, taskStatusInProgress, nil, nil); err != nil {
		t.Fatalf("update без результата: %v", err)
	}
	if got, _ := appTasks.getOwned("admin", task.ID); got.Result != res || got.Log != logText {
		t.Fatalf("update с nil затёр result/log: %q / %q", got.Result, got.Log)
	}

	// Приватность: другой пользователь не видит и не трогает чужие задачи.
	if tasks := appTasks.list("other"); len(tasks) != 0 {
		t.Fatalf("list(other) = %d задач, want 0", len(tasks))
	}
	if _, err := appTasks.update("other", task.ID, "Чужая", "", taskStatusDone, nil, nil); err == nil ||
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
