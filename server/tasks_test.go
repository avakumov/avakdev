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
	goals = &goalStore{
		data:   make(map[int]Goal),
		nextID: 1,
		hasDB:  false,
	}

	// Создание с полными полями.
	task, err := tasks.create("admin", "Работа", "Сверстать таблицу", "Таблица для десктопа, карточки для мобильных", 4, 0, "2026-09-01", taskTodo, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Category != "Работа" || task.PlannedHours != 4 || task.Deadline != "2026-09-01" || task.Status != taskTodo || task.GoalID != nil {
		t.Fatalf("поля задачи не сохранились: %+v", task)
	}

	// Пустая категория подставляется «Прочее».
	other, err := tasks.create("admin", "", "Без категории", "", 0, 0, "", taskTodo, nil)
	if err != nil {
		t.Fatalf("create без категории: %v", err)
	}
	if other.Category != "Прочее" {
		t.Fatalf("category = %q, want Прочее", other.Category)
	}

	// Пустой заголовок отклоняется.
	if _, err := tasks.create("admin", "Работа", "   ", "", 0, 0, "", taskTodo, nil); err == nil {
		t.Fatal("create с пустым заголовком должен падать")
	}

	// Некорректный статус отклоняется.
	if _, err := tasks.create("admin", "Работа", "Задача", "", 0, 0, "", "banana", nil); err == nil {
		t.Fatal("create с неизвестным статусом должен падать")
	}

	// Отрицательное время отклоняется.
	if _, err := tasks.create("admin", "Работа", "Задача", "", -1, 0, "", taskTodo, nil); err == nil {
		t.Fatal("create с отрицательным временем должен падать")
	}

	// Ссылка на несуществующую/чужую цель отклоняется.
	missingGoal := 999
	if _, err := tasks.create("admin", "Работа", "Задача", "", 0, 0, "", taskTodo, &missingGoal); err == nil {
		t.Fatal("create с несуществующей целью должен падать")
	}

	// Создание с привязкой к существующей цели пользователя.
	goal, err := goals.create("admin", "Пробежать марафон", "", "2026-12-31", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	linked, err := tasks.create("admin", "Работа", "Тренировка", "", 1, 0, "", taskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("create с целью: %v", err)
	}
	if linked.GoalID == nil || *linked.GoalID != goal.ID {
		t.Fatalf("goal_id не сохранился: %+v", linked)
	}
	// Чужая цель не привязывается (цель принадлежит другому пользователю).
	otherGoal, err := goals.create("other", "Чужая цель", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal other: %v", err)
	}
	if _, err := tasks.create("admin", "Работа", "Задача", "", 0, 0, "", taskTodo, &otherGoal.ID); err == nil ||
		!strings.Contains(err.Error(), "цель не найдена") {
		t.Fatalf("create с целью другого пользователя должен падать: %v", err)
	}

	// Обновление.
	updated, err := tasks.update("admin", task.ID, "Личное", "Новый заголовок", "Новое описание", 2, 1.5, "", taskInProgress, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "Новый заголовок" || updated.Category != "Личное" ||
		updated.PlannedHours != 2 || updated.ActualHours != 1.5 ||
		updated.Status != taskInProgress || updated.Deadline != "" {
		t.Fatalf("update не применился: %+v", updated)
	}

	// Привязка к цели при обновлении и отвязка (nil).
	linked2, err := tasks.update("admin", other.ID, other.Category, other.Title, other.Description, 0, 0, "", taskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("update с целью: %v", err)
	}
	if linked2.GoalID == nil || *linked2.GoalID != goal.ID {
		t.Fatalf("update не привязал цель: %+v", linked2)
	}
	unlinked, err := tasks.update("admin", other.ID, other.Category, other.Title, other.Description, 0, 0, "", taskTodo, nil)
	if err != nil {
		t.Fatalf("update без цели: %v", err)
	}
	if unlinked.GoalID != nil {
		t.Fatalf("update не отвязал цель: %+v", unlinked)
	}

	// Приватность: другой пользователь не видит и не трогает чужие задачи.
	if list := tasks.list("other"); len(list) != 0 {
		t.Fatalf("list(other) = %d задач, want 0", len(list))
	}
	if _, err := tasks.update("other", task.ID, "Работа", "Чужая", "", 0, 0, "", taskTodo, nil); err == nil ||
		!strings.Contains(err.Error(), "не найдена") {
		t.Fatalf("update чужой задачи должен падать: %v", err)
	}
	if err := tasks.delete("other", task.ID); err == nil {
		t.Fatal("delete чужой задачи должен падать")
	}

	// Сортировка: новые сверху.
	list := tasks.list("admin")
	if len(list) != 3 || list[0].ID < list[1].ID {
		t.Fatalf("list должен быть отсортирован по убыванию ID: %+v", list)
	}

	// Удаление цели сбрасывает ссылки задач (аналог ON DELETE SET NULL).
	if err := goals.delete("admin", goal.ID); err != nil {
		t.Fatalf("delete goal: %v", err)
	}
	if got, ok := tasks.getOwned("admin", linked.ID); ok && got.GoalID != nil {
		t.Fatalf("goal_id должен сброситься после удаления цели: %+v", got)
	}

	// Удаление.
	if err := tasks.delete("admin", task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := tasks.getOwned("admin", task.ID); ok {
		t.Fatal("задача должна исчезнуть после delete")
	}
}
