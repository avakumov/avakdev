package main

import (
	"strings"
	"testing"
)

// Тест хранилища задач раздела «Задачи» (in-memory, без БД):
// создание с категорией/временем/дедлайном, валидация, приватность.
func TestTaskStore(t *testing.T) {
	tasks = &taskStore{
		data:   make(map[int]Task),
		nextID: 1,
		hasDB:  false,
	}

	// Создание с полными полями.
	task, err := tasks.create("admin", "Работа", "Сверстать таблицу", "Таблица для десктопа, карточки для мобильных", 4, 0, "2026-09-01", taskTodo)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Category != "Работа" || task.PlannedHours != 4 || task.Deadline != "2026-09-01" || task.Status != taskTodo {
		t.Fatalf("поля задачи не сохранились: %+v", task)
	}

	// Пустая категория подставляется «Прочее».
	other, err := tasks.create("admin", "", "Без категории", "", 0, 0, "", taskTodo)
	if err != nil {
		t.Fatalf("create без категории: %v", err)
	}
	if other.Category != "Прочее" {
		t.Fatalf("category = %q, want Прочее", other.Category)
	}

	// Пустой заголовок отклоняется.
	if _, err := tasks.create("admin", "Работа", "   ", "", 0, 0, "", taskTodo); err == nil {
		t.Fatal("create с пустым заголовком должен падать")
	}

	// Некорректный статус отклоняется.
	if _, err := tasks.create("admin", "Работа", "Задача", "", 0, 0, "", "banana"); err == nil {
		t.Fatal("create с неизвестным статусом должен падать")
	}

	// Отрицательное время отклоняется.
	if _, err := tasks.create("admin", "Работа", "Задача", "", -1, 0, "", taskTodo); err == nil {
		t.Fatal("create с отрицательным временем должен падать")
	}

	// Обновление.
	updated, err := tasks.update("admin", task.ID, "Личное", "Новый заголовок", "Новое описание", 2, 1.5, "", taskInProgress)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "Новый заголовок" || updated.Category != "Личное" ||
		updated.PlannedHours != 2 || updated.ActualHours != 1.5 ||
		updated.Status != taskInProgress || updated.Deadline != "" {
		t.Fatalf("update не применился: %+v", updated)
	}

	// Приватность: другой пользователь не видит и не трогает чужие задачи.
	if list := tasks.list("other"); len(list) != 0 {
		t.Fatalf("list(other) = %d задач, want 0", len(list))
	}
	if _, err := tasks.update("other", task.ID, "Работа", "Чужая", "", 0, 0, "", taskTodo); err == nil ||
		!strings.Contains(err.Error(), "не найдена") {
		t.Fatalf("update чужой задачи должен падать: %v", err)
	}
	if err := tasks.delete("other", task.ID); err == nil {
		t.Fatal("delete чужой задачи должен падать")
	}

	// Сортировка: новые сверху.
	list := tasks.list("admin")
	if len(list) != 2 || list[0].ID < list[1].ID {
		t.Fatalf("list должен быть отсортирован по убыванию ID: %+v", list)
	}

	// Удаление.
	if err := tasks.delete("admin", task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := tasks.getOwned("admin", task.ID); ok {
		t.Fatal("задача должна исчезнуть после delete")
	}
}
