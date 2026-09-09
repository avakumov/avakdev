package main

import (
	"testing"
	"time"
)

// Время повторения заметки считается по символам (≈ 1500 симв/мин).
func TestReadingMinutes(t *testing.T) {
	speed := 1500
	cases := map[string]int{
		"":              1,
		strings15(10):   1,
		strings15(1499): 1,
		strings15(1500): 1,
		strings15(1501): 2,
		strings15(4500): 3,
	}
	for in, want := range cases {
		if got := readingMinutes(in, speed); got != want {
			t.Fatalf("readingMinutes(len=%d) = %d, want %d", len([]rune(in)), got, want)
		}
	}
}

func strings15(n int) string {
	r := make([]rune, n)
	for i := range r {
		r[i] = 'а'
	}
	return string(r)
}

// Даты повторения считаются от даты создания: 0 — свежая заметка доступна
// сразу, затем +1, +2, +4, … дней; после прохождения графика — 60 дней
// от последнего повторения.
func TestNextRepeatDays(t *testing.T) {
	want := []int{0, 1, 2, 4, 7, 14, 30, 60}
	for n, exp := range want {
		if got := nextRepeatDays(n); got != exp {
			t.Fatalf("nextRepeatDays(%d) = %d, want %d", n, got, exp)
		}
	}
	if got := nextRepeatDays(100); got != 60 {
		t.Fatalf("nextRepeatDays(100) = %d, want 60", got)
	}
}

// Свежесозданная заметка (0 повторений) всегда попадает в «к повторению»,
// даже если её created в БД оказался позже формируемого дня (уход часов БД
// вперёд / пересечение полуночи).
func TestDueKnowledgeFreshNote(t *testing.T) {
	notes = newNoteStore()
	n, err := notes.create("тема", "заголовок", strings15(30))
	if err != nil {
		t.Fatalf("notes.create: %v", err)
	}
	// Имитируем «убежавшие» часы БД: created в будущем относительно дня.
	n.Created = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	notes.data[n.ID] = n

	day := time.Now().Format("2006-01-02")
	found := false
	for _, d := range dueKnowledgeNotes(day) {
		if d.ID == n.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("свежая заметка не попала в список повторений на сегодня")
	}
}

// Срок повторения отсчитывается от даты создания заметки, а не от даты
// последнего повторения.
func TestNoteDueDate(t *testing.T) {
	created := "2026-09-01T10:00:00Z"
	cases := []struct {
		reps    int
		updated string
		want    string
	}{
		{0, created, "2026-09-01T10:00:00Z"}, // свежая — сразу
		{1, created, "2026-09-02T10:00:00Z"}, // повторение на 1-й день
		{2, created, "2026-09-03T10:00:00Z"}, // на 2-й день
		{3, created, "2026-09-05T10:00:00Z"}, // на 4-й день
		{4, created, "2026-09-08T10:00:00Z"}, // на 7-й день
		{5, created, "2026-09-15T10:00:00Z"}, // на 14-й день
		{6, created, "2026-10-01T10:00:00Z"}, // на 30-й день
		{7, created, "2026-10-31T10:00:00Z"}, // на 60-й день
		// График пройден: дальше 60 дней от последнего повторения.
		{8, "2026-10-31T10:00:00Z", "2026-12-30T10:00:00Z"},
	}
	for _, tc := range cases {
		due, err := noteDueDate(created, tc.updated, tc.reps)
		if err != nil {
			t.Fatalf("noteDueDate(%d): %v", tc.reps, err)
		}
		want, err := time.Parse(time.RFC3339, tc.want)
		if err != nil {
			t.Fatalf("bad want %q: %v", tc.want, err)
		}
		if !due.Equal(want) {
			t.Fatalf("reps=%d: due = %v, want %v", tc.reps, due.Format(time.RFC3339), tc.want)
		}
	}
}
