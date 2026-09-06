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
	if err := goals.delete("admin", goal.ID); err != nil {
		t.Fatalf("delete goal: %v", err)
	}
	snapshot.computeProgress()
	if got := snapshot.Progress; got != 0 {
		t.Fatalf("прогресс после удаления цели = %d%%, want 0", got)
	}
}
