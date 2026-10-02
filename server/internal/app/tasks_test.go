package app

import (
	"strings"
	"testing"
)

// newTestTaskGoalStores создаёт связанную пару in-memory хранилищ задач и целей.
func newTestTaskGoalStores() (*TaskStore, *GoalStore) {
	ts := &TaskStore{data: make(map[int]Task), nextID: 1}
	gs := &GoalStore{data: make(map[int]Goal), nextID: 1}
	ts.goals = gs
	gs.tasks = ts
	return ts, gs
}

// Тест хранилища задач раздела «Задачи» (in-memory, без БД):
// создание с категорией/временем/дедлайном, валидация, приватность.
func TestTaskStore(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	// Создание с полными полями.
	task, err := tasks.Create("admin", "Работа", "Сверстать таблицу", "Таблица для десктопа, карточки для мобильных", 4, "2026-09-01", TaskTodo, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Category != "Работа" || task.PlannedHours != 4 || task.Deadline != "2026-09-01" || task.Status != TaskTodo || task.GoalID != nil {
		t.Fatalf("поля задачи не сохранились: %+v", task)
	}

	// Пустая категория подставляется «Прочее».
	other, err := tasks.Create("admin", "", "Без категории", "", 0, "", TaskTodo, nil)
	if err != nil {
		t.Fatalf("create без категории: %v", err)
	}
	if other.Category != "Прочее" {
		t.Fatalf("category = %q, want Прочее", other.Category)
	}

	// Пустой заголовок отклоняется.
	if _, err := tasks.Create("admin", "Работа", "   ", "", 0, "", TaskTodo, nil); err == nil {
		t.Fatal("create с пустым заголовком должен падать")
	}

	// Некорректный статус отклоняется.
	if _, err := tasks.Create("admin", "Работа", "Задача", "", 0, "", "banana", nil); err == nil {
		t.Fatal("create с неизвестным статусом должен падать")
	}

	// Отрицательное время отклоняется.
	if _, err := tasks.Create("admin", "Работа", "Задача", "", -1, "", TaskTodo, nil); err == nil {
		t.Fatal("create с отрицательным временем должен падать")
	}

	// Ссылка на несуществующую/чужую цель отклоняется.
	missingGoal := 999
	if _, err := tasks.Create("admin", "Работа", "Задача", "", 0, "", TaskTodo, &missingGoal); err == nil {
		t.Fatal("create с несуществующей целью должен падать")
	}

	// Создание с привязкой к существующей цели пользователя.
	goal, err := goals.Create("admin", "Пробежать марафон", "", "2026-12-31", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	linked, err := tasks.Create("admin", "Работа", "Тренировка", "", 1, "", TaskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("create с целью: %v", err)
	}
	if linked.GoalID == nil || *linked.GoalID != goal.ID {
		t.Fatalf("goal_id не сохранился: %+v", linked)
	}
	// Чужая цель не привязывается (цель принадлежит другому пользователю).
	otherGoal, err := goals.Create("other", "Чужая цель", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal other: %v", err)
	}
	if _, err := tasks.Create("admin", "Работа", "Задача", "", 0, "", TaskTodo, &otherGoal.ID); err == nil ||
		!strings.Contains(err.Error(), "цель не найдена") {
		t.Fatalf("create с целью другого пользователя должен падать: %v", err)
	}

	// Обновление.
	updated, err := tasks.Update("admin", task.ID, "Личное", "Новый заголовок", "Новое описание", 2, "", TaskInProgress, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "Новый заголовок" || updated.Category != "Личное" ||
		updated.PlannedHours != 2 ||
		updated.Status != TaskInProgress || updated.Deadline != "" {
		t.Fatalf("update не применился: %+v", updated)
	}

	// Привязка к цели при обновлении и отвязка (nil).
	linked2, err := tasks.Update("admin", other.ID, other.Category, other.Title, other.Description, 0, "", TaskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("update с целью: %v", err)
	}
	if linked2.GoalID == nil || *linked2.GoalID != goal.ID {
		t.Fatalf("update не привязал цель: %+v", linked2)
	}
	unlinked, err := tasks.Update("admin", other.ID, other.Category, other.Title, other.Description, 0, "", TaskTodo, nil)
	if err != nil {
		t.Fatalf("update без цели: %v", err)
	}
	if unlinked.GoalID != nil {
		t.Fatalf("update не отвязал цель: %+v", unlinked)
	}

	// Приватность: другой пользователь не видит и не трогает чужие задачи.
	if list := tasks.List("other"); len(list) != 0 {
		t.Fatalf("list(other) = %d задач, want 0", len(list))
	}
	if _, err := tasks.Update("other", task.ID, "Работа", "Чужая", "", 0, "", TaskTodo, nil); err == nil ||
		!strings.Contains(err.Error(), "не найдена") {
		t.Fatalf("update чужой задачи должен падать: %v", err)
	}
	if err := tasks.Delete("other", task.ID); err == nil {
		t.Fatal("delete чужой задачи должен падать")
	}

	// Сортировка: новые сверху.
	list := tasks.List("admin")
	if len(list) != 3 || list[0].ID < list[1].ID {
		t.Fatalf("list должен быть отсортирован по убыванию ID: %+v", list)
	}

	// Удаление цели сбрасывает ссылки задач (аналог ON DELETE SET NULL).
	if err := goals.Delete("admin", goal.ID, false); err != nil {
		t.Fatalf("delete goal: %v", err)
	}
	if got, ok := tasks.GetOwned("admin", linked.ID); ok && got.GoalID != nil {
		t.Fatalf("goal_id должен сброситься после удаления цели: %+v", got)
	}

	// Удаление.
	if err := tasks.Delete("admin", task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := tasks.GetOwned("admin", task.ID); ok {
		t.Fatal("задача должна исчезнуть после delete")
	}
}

// При deleteTasks=true удаление цели удаляет и её задачи (из БД и памяти);
// задачи других пользователей и чужие цели не затрагиваются.
func TestGoalDeleteWithTasks(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	goal, err := goals.Create("admin", "Цель каскад", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	t1, err := tasks.Create("admin", "Работа", "Задача 1", "", 1, "", TaskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("create task 1: %v", err)
	}
	t2, err := tasks.Create("admin", "Работа", "Задача 2", "", 2, "", TaskDone, &goal.ID)
	if err != nil {
		t.Fatalf("create task 2: %v", err)
	}
	// Задачи других пользователей не затрагиваются.
	other, err := tasks.Create("other", "Личное", "Чужая задача", "", 1, "", TaskTodo, nil)
	if err != nil {
		t.Fatalf("create other task: %v", err)
	}

	deleted, err := tasks.RemoveByGoal("admin", goal.ID)
	if err != nil {
		t.Fatalf("removeByGoal: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("removeByGoal удалила %d задач, want 2", deleted)
	}
	if _, ok := tasks.GetOwned("admin", t1.ID); ok {
		t.Fatal("задача 1 должна быть удалена")
	}
	if _, ok := tasks.GetOwned("admin", t2.ID); ok {
		t.Fatal("задача 2 должна быть удалена")
	}
	if _, ok := tasks.GetOwned("other", other.ID); !ok {
		t.Fatal("чужая задача не должна быть удалена")
	}

	// Цель ещё существует (удаляем задачи отдельно от цели).
	if _, ok := goals.GetOwned("admin", goal.ID); !ok {
		t.Fatal("цель должна остаться после removeByGoal")
	}
}

// Порядок задач внутри цели: новые задачи дописываются в конец (позиции 1..N),
// SetGoalOrder меняет последовательность, перенос задачи в цель дописывает её.
func TestTaskGoalOrder(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	goal, err := goals.Create("admin", "Цель", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	var ids []int
	for _, name := range []string{"Первый", "Второй", "Третий"} {
		tk, err := tasks.Create("admin", "Работа", name, "", 1, "", TaskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}

	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.GetOwned("admin", id)
		if !ok {
			t.Fatalf("задача %d не найдена", id)
		}
		return g.Position
	}
	// Новые задачи получили позиции 1, 2, 3 в порядке создания.
	for i, id := range ids {
		if got := pos(id); got != i+1 {
			t.Fatalf("position задачи %d = %d, want %d", id, got, i+1)
		}
	}

	// Перестановка: Третий, Первый, Второй.
	reordered := []int{ids[2], ids[0], ids[1]}
	if err := tasks.SetGoalOrder("admin", goal.ID, reordered); err != nil {
		t.Fatalf("setGoalOrder: %v", err)
	}
	for i, id := range reordered {
		if got := pos(id); got != i+1 {
			t.Fatalf("после перестановки position задачи %d = %d, want %d", id, got, i+1)
		}
	}

	// Неполный список отклоняется.
	if err := tasks.SetGoalOrder("admin", goal.ID, ids[:2]); err == nil {
		t.Fatal("setGoalOrder с неполным списком должен падать")
	}
	// Список с чужой задачей отклоняется.
	other, err := tasks.Create("other", "Работа", "Чужая", "", 1, "", TaskTodo, nil)
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := tasks.SetGoalOrder("admin", goal.ID, append(ids, other.ID)); err == nil {
		t.Fatal("setGoalOrder с чужой задачей должен падать")
	}

	// Задача без цели при переносе в цель дописывается в конец (позиция 4).
	free, err := tasks.Create("admin", "Работа", "Свободная", "", 1, "", TaskTodo, nil)
	if err != nil {
		t.Fatalf("create free: %v", err)
	}
	if got := pos(free.ID); got != 0 {
		t.Fatalf("position задачи без цели = %d, want 0", got)
	}
	if _, err := tasks.Update("admin", free.ID, free.Category, free.Title, free.Description,
		free.PlannedHours, free.Deadline, TaskTodo, &goal.ID); err != nil {
		t.Fatalf("attach free: %v", err)
	}
	if got := pos(free.ID); got != 4 {
		t.Fatalf("position после переноса в цель = %d, want 4", got)
	}
}

// Удаление задачи цели не оставляет «дырок» в последовательности:
// оставшиеся задачи перенумеровываются подряд (1..N).
func TestTaskGoalDeleteCompaction(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	goal, err := goals.Create("admin", "Цель", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	ids := make([]int, 0, 3)
	for _, name := range []string{"Подготовка", "Тренировка", "Соревнование"} {
		tk, err := tasks.Create("admin", "Работа", name, "", 1, "", TaskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}
	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.GetOwned("admin", id)
		if !ok {
			t.Fatalf("задача %d не найдена", id)
		}
		return g.Position
	}

	// Удаляем среднюю задачу (позиция 2).
	if err := tasks.Delete("admin", ids[1]); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := pos(ids[0]); got != 1 {
		t.Fatalf("position первой = %d, want 1", got)
	}
	// Третья задача «съехала» на позицию 2 — без дырки.
	if got := pos(ids[2]); got != 2 {
		t.Fatalf("position третьей после удаления = %d, want 2", got)
	}

	// Удаляем первую — остаётся одна задача с позицией 1.
	if err := tasks.Delete("admin", ids[0]); err != nil {
		t.Fatalf("delete first: %v", err)
	}
	if got := pos(ids[2]); got != 1 {
		t.Fatalf("position последней после удалений = %d, want 1", got)
	}
}

// Отвязка задачи от цели (или перенос в другую цель) тоже уплотняет
// позиции оставшихся задач прежней цели; сама отвязанная задача получает 0.
func TestTaskGoalUnlinkCompaction(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	goal, err := goals.Create("admin", "Цель", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	ids := make([]int, 0, 3)
	for _, name := range []string{"Подготовка", "Тренировка", "Соревнование"} {
		tk, err := tasks.Create("admin", "Работа", name, "", 1, "", TaskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}
	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.GetOwned("admin", id)
		if !ok {
			t.Fatalf("задача %d не найдена", id)
		}
		return g.Position
	}

	// Отвязываем среднюю задачу (goal_id → nil).
	mid, ok := tasks.GetOwned("admin", ids[1])
	if !ok {
		t.Fatalf("задача %d не найдена", ids[1])
	}
	if _, err := tasks.Update("admin", mid.ID, mid.Category, mid.Title, mid.Description,
		mid.PlannedHours, mid.Deadline, TaskTodo, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if got := pos(ids[1]); got != 0 {
		t.Fatalf("position отвязанной задачи = %d, want 0", got)
	}
	if got := pos(ids[0]); got != 1 {
		t.Fatalf("position первой = %d, want 1", got)
	}
	if got := pos(ids[2]); got != 2 {
		t.Fatalf("position третьей после отвязки = %d, want 2", got)
	}
}
