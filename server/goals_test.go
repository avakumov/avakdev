package main

import "testing"

// Прогресс цели считается на лету из привязанных задач: доля выполненных
// задач среди неотменённых. Все задачи выполнены → 99%; 100% — только
// после перевода цели в статус «достигнута» (achieved).
func TestGoalAutoProgress(t *testing.T) {
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

	goal, err := goals.create("admin", "Цель", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// progressOf перечитывает цель из хранилища: статус меняется через update.
	progressOf := func() int {
		t.Helper()
		g, ok := goals.getOwned("admin", goal.ID)
		if !ok {
			t.Fatalf("цель %d не найдена", goal.ID)
		}
		g.computeProgress()
		return g.Progress
	}

	// Без задач — 0%.
	if got := progressOf(); got != 0 {
		t.Fatalf("прогресс без задач = %d, want 0", got)
	}

	add := func(status string) {
		t.Helper()
		if _, err := tasks.create("admin", "Прочее", "Задача", "", 1, 0, "", status, &goal.ID); err != nil {
			t.Fatalf("create task %q: %v", status, err)
		}
	}

	add(taskDone) // все задачи выполнены, но цель активна → 99%
	if got := progressOf(); got != 99 {
		t.Fatalf("1/1 при активной цели = %d%%, want 99", got)
	}

	add(taskInProgress) // 1 из 2
	if got := progressOf(); got != 50 {
		t.Fatalf("1/2 = %d%%, want 50", got)
	}

	add(taskTodo) // 1 из 3, округление 33
	if got := progressOf(); got != 33 {
		t.Fatalf("1/3 = %d%%, want 33", got)
	}

	add(taskCancelled) // отменённая не учитывается: по-прежнему 1 из 3
	if got := progressOf(); got != 33 {
		t.Fatalf("1/3 + cancelled = %d%%, want 33", got)
	}

	add(taskDone) // 2 из 4 (cancelled исключена из подсчёта)
	if got := progressOf(); got != 50 {
		t.Fatalf("2/4 = %d%%, want 50", got)
	}

	// 100% только после перевода цели в «достигнута».
	if _, err := goals.update("admin", goal.ID, goal.Title, goal.Description, goal.TargetDate, goalAchieved); err != nil {
		t.Fatalf("update goal -> achieved: %v", err)
	}
	if got := progressOf(); got != 100 {
		t.Fatalf("achieved = %d%%, want 100", got)
	}

	// Возврат в активную: снова обычный расчёт (2 из 4 → 50).
	if _, err := goals.update("admin", goal.ID, goal.Title, goal.Description, goal.TargetDate, goalActive); err != nil {
		t.Fatalf("update goal -> active: %v", err)
	}
	if got := progressOf(); got != 50 {
		t.Fatalf("active после achieved = %d%%, want 50", got)
	}

	// Задачи без цели не влияют на прогресс.
	if _, err := tasks.create("admin", "Прочее", "Свободная", "", 1, 0, "", taskDone, nil); err != nil {
		t.Fatalf("create task без цели: %v", err)
	}
	if got := progressOf(); got != 50 {
		t.Fatalf("прогресс с посторонней задачей = %d%%, want 50", got)
	}

	// Удаление цели сбрасывает ссылки задач → прогресс снова 0.
	snapshot, _ := goals.getOwned("admin", goal.ID)
	if err := goals.delete("admin", goal.ID, false); err != nil {
		t.Fatalf("delete goal: %v", err)
	}
	snapshot.computeProgress()
	if got := snapshot.Progress; got != 0 {
		t.Fatalf("прогресс после удаления цели = %d%%, want 0", got)
	}
}

// Разбор JSON-ответа DeepSeek в черновики задач: вырезание ```json-обёртки,
// нормализация категории/часов, отбрасывание пустых заголовков.
func TestParseGoalTaskDrafts(t *testing.T) {
	content := "```json\n" +
		"{\"tasks\": [" +
		"{\"title\": \"Составить план тренировок\", \"description\": \"На первую неделю\", \"category\": \"Личное\", \"planned_hours\": 2}," +
		"{\"title\": \"Купить кроссовки\", \"description\": \"\", \"category\": \"Магазин\", \"planned_hours\": -1}," +
		"{\"title\": \"   \", \"description\": \"Пустая\", \"category\": \"Работа\", \"planned_hours\": 1}" +
		"]}"

	drafts, err := parseGoalTaskDrafts(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("получено %d черновиков, want 2: %+v", len(drafts), drafts)
	}
	if drafts[0].Title != "Составить план тренировок" || drafts[0].Category != "Личное" || drafts[0].PlannedHours != 2 {
		t.Fatalf("первый черновик нормализован неверно: %+v", drafts[0])
	}
	// Неизвестная категория → «Прочее», отрицательные часы → 0.
	if drafts[1].Category != "Прочее" || drafts[1].PlannedHours != 0 {
		t.Fatalf("второй черновик нормализован неверно: %+v", drafts[1])
	}
}

// При deleteTasks=true удаление цели удаляет и её задачи (из БД и памяти);
// задачи других пользователей и чужие цели не затрагиваются.
func TestGoalDeleteWithTasks(t *testing.T) {
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

	goal, err := goals.create("admin", "Цель каскад", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	t1, err := tasks.create("admin", "Работа", "Задача 1", "", 1, 0, "", taskTodo, &goal.ID)
	if err != nil {
		t.Fatalf("create task 1: %v", err)
	}
	t2, err := tasks.create("admin", "Работа", "Задача 2", "", 2, 0, "", taskDone, &goal.ID)
	if err != nil {
		t.Fatalf("create task 2: %v", err)
	}
	// Задачи других пользователей не затрагиваются.
	other, err := tasks.create("other", "Личное", "Чужая задача", "", 1, 0, "", taskTodo, nil)
	if err != nil {
		t.Fatalf("create other task: %v", err)
	}

	deleted, err := tasks.removeByGoal("admin", goal.ID)
	if err != nil {
		t.Fatalf("removeByGoal: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("removeByGoal удалила %d задач, want 2", deleted)
	}
	if _, ok := tasks.getOwned("admin", t1.ID); ok {
		t.Fatal("задача 1 должна быть удалена")
	}
	if _, ok := tasks.getOwned("admin", t2.ID); ok {
		t.Fatal("задача 2 должна быть удалена")
	}
	if _, ok := tasks.getOwned("other", other.ID); !ok {
		t.Fatal("чужая задача не должна быть удалена")
	}

	// Цель ещё существует (удаляем задачи отдельно от цели).
	if _, ok := goals.getOwned("admin", goal.ID); !ok {
		t.Fatal("цель должна остаться после removeByGoal")
	}
}

// Порядок задач внутри цели: новые задачи дописываются в конец (позиции 1..N),
// setGoalOrder меняет последовательность, перенос задачи в цель дописывает её.
func TestTaskGoalOrder(t *testing.T) {
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

	goal, err := goals.create("admin", "Цель", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	var ids []int
	for _, name := range []string{"Первый", "Второй", "Третий"} {
		tk, err := tasks.create("admin", "Работа", name, "", 1, 0, "", taskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}

	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.getOwned("admin", id)
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
	if err := tasks.setGoalOrder("admin", goal.ID, reordered); err != nil {
		t.Fatalf("setGoalOrder: %v", err)
	}
	for i, id := range reordered {
		if got := pos(id); got != i+1 {
			t.Fatalf("после перестановки position задачи %d = %d, want %d", id, got, i+1)
		}
	}

	// Неполный список отклоняется.
	if err := tasks.setGoalOrder("admin", goal.ID, ids[:2]); err == nil {
		t.Fatal("setGoalOrder с неполным списком должен падать")
	}
	// Список с чужой задачей отклоняется.
	other, err := tasks.create("other", "Работа", "Чужая", "", 1, 0, "", taskTodo, nil)
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := tasks.setGoalOrder("admin", goal.ID, append(ids, other.ID)); err == nil {
		t.Fatal("setGoalOrder с чужой задачей должен падать")
	}

	// Задача без цели при переносе в цель дописывается в конец (позиция 4).
	free, err := tasks.create("admin", "Работа", "Свободная", "", 1, 0, "", taskTodo, nil)
	if err != nil {
		t.Fatalf("create free: %v", err)
	}
	if got := pos(free.ID); got != 0 {
		t.Fatalf("position задачи без цели = %d, want 0", got)
	}
	if _, err := tasks.update("admin", free.ID, free.Category, free.Title, free.Description,
		free.PlannedHours, free.ActualHours, free.Deadline, taskTodo, &goal.ID); err != nil {
		t.Fatalf("attach free: %v", err)
	}
	if got := pos(free.ID); got != 4 {
		t.Fatalf("position после переноса в цель = %d, want 4", got)
	}
}

// Удаление задачи цели не оставляет «дырок» в последовательности:
// оставшиеся задачи перенумеровываются подряд (1..N).
func TestTaskGoalDeleteCompaction(t *testing.T) {
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

	goal, err := goals.create("admin", "Цель", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	ids := make([]int, 0, 3)
	for _, name := range []string{"Подготовка", "Тренировка", "Соревнование"} {
		tk, err := tasks.create("admin", "Работа", name, "", 1, 0, "", taskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}
	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.getOwned("admin", id)
		if !ok {
			t.Fatalf("задача %d не найдена", id)
		}
		return g.Position
	}

	// Удаляем среднюю задачу (позиция 2).
	if err := tasks.delete("admin", ids[1]); err != nil {
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
	if err := tasks.delete("admin", ids[0]); err != nil {
		t.Fatalf("delete first: %v", err)
	}
	if got := pos(ids[2]); got != 1 {
		t.Fatalf("position последней после удалений = %d, want 1", got)
	}
}

// Отвязка задачи от цели (или перенос в другую цель) тоже уплотняет
// позиции оставшихся задач прежней цели; сама отвязанная задача получает 0.
func TestTaskGoalUnlinkCompaction(t *testing.T) {
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

	goal, err := goals.create("admin", "Цель", "", "", goalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	ids := make([]int, 0, 3)
	for _, name := range []string{"Подготовка", "Тренировка", "Соревнование"} {
		tk, err := tasks.create("admin", "Работа", name, "", 1, 0, "", taskTodo, &goal.ID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, tk.ID)
	}
	pos := func(id int) int {
		t.Helper()
		g, ok := tasks.getOwned("admin", id)
		if !ok {
			t.Fatalf("задача %d не найдена", id)
		}
		return g.Position
	}

	// Отвязываем среднюю задачу (goal_id → nil).
	mid, ok := tasks.getOwned("admin", ids[1])
	if !ok {
		t.Fatalf("задача %d не найдена", ids[1])
	}
	if _, err := tasks.update("admin", mid.ID, mid.Category, mid.Title, mid.Description,
		mid.PlannedHours, mid.ActualHours, mid.Deadline, taskTodo, nil); err != nil {
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
