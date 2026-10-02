package app

import "testing"

// Прогресс цели считается на лету из привязанных задач: доля выполненных
// задач среди неотменённых. Все задачи выполнены → 99%; 100% — только
// после перевода цели в статус «достигнута» (achieved).
func TestGoalAutoProgress(t *testing.T) {
	tasks, goals := newTestTaskGoalStores()

	goal, err := goals.Create("admin", "Цель", "", "", GoalActive)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// progressOf перечитывает цель из хранилища: статус меняется через update.
	progressOf := func() int {
		t.Helper()
		g, ok := goals.GetOwned("admin", goal.ID)
		if !ok {
			t.Fatalf("цель %d не найдена", goal.ID)
		}
		goals.ComputeProgress(&g)
		return g.Progress
	}

	// Без задач — 0%.
	if got := progressOf(); got != 0 {
		t.Fatalf("прогресс без задач = %d, want 0", got)
	}

	add := func(status string) {
		t.Helper()
		if _, err := tasks.Create("admin", "Прочее", "Задача", "", 1, "", status, &goal.ID); err != nil {
			t.Fatalf("create task %q: %v", status, err)
		}
	}

	add(TaskDone) // все задачи выполнены, но цель активна → 99%
	if got := progressOf(); got != 99 {
		t.Fatalf("1/1 при активной цели = %d%%, want 99", got)
	}

	add(TaskInProgress) // 1 из 2
	if got := progressOf(); got != 50 {
		t.Fatalf("1/2 = %d%%, want 50", got)
	}

	add(TaskTodo) // 1 из 3, округление 33
	if got := progressOf(); got != 33 {
		t.Fatalf("1/3 = %d%%, want 33", got)
	}

	add(TaskCancelled) // отменённая не учитывается: по-прежнему 1 из 3
	if got := progressOf(); got != 33 {
		t.Fatalf("1/3 + cancelled = %d%%, want 33", got)
	}

	add(TaskDone) // 2 из 4 (cancelled исключена из подсчёта)
	if got := progressOf(); got != 50 {
		t.Fatalf("2/4 = %d%%, want 50", got)
	}

	// 100% только после перевода цели в «достигнута».
	if _, err := goals.Update("admin", goal.ID, goal.Title, goal.Description, goal.TargetDate, GoalAchieved); err != nil {
		t.Fatalf("update goal -> achieved: %v", err)
	}
	if got := progressOf(); got != 100 {
		t.Fatalf("achieved = %d%%, want 100", got)
	}

	// Возврат в активную: снова обычный расчёт (2 из 4 → 50).
	if _, err := goals.Update("admin", goal.ID, goal.Title, goal.Description, goal.TargetDate, GoalActive); err != nil {
		t.Fatalf("update goal -> active: %v", err)
	}
	if got := progressOf(); got != 50 {
		t.Fatalf("active после achieved = %d%%, want 50", got)
	}

	// Задачи без цели не влияют на прогресс.
	if _, err := tasks.Create("admin", "Прочее", "Свободная", "", 1, "", TaskDone, nil); err != nil {
		t.Fatalf("create task без цели: %v", err)
	}
	if got := progressOf(); got != 50 {
		t.Fatalf("прогресс с посторонней задачей = %d%%, want 50", got)
	}

	// Удаление цели сбрасывает ссылки задач → прогресс снова 0.
	snapshot, _ := goals.GetOwned("admin", goal.ID)
	if err := goals.Delete("admin", goal.ID, false); err != nil {
		t.Fatalf("delete goal: %v", err)
	}
	goals.ComputeProgress(&snapshot)
	if got := snapshot.Progress; got != 0 {
		t.Fatalf("прогресс после удаления цели = %d%%, want 0", got)
	}
}
