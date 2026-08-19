package main

import (
	"testing"
)

// Тест логики «важного» сообщения на in-memory хранилище (без БД):
// персональность сообщений и отметка прочтения «раз в сутки».
func TestImportantStore(t *testing.T) {
	important = &importantStore{
		hasDB:    false,
		messages: make(map[string]importantMessage),
		seen:     make(map[string]string),
	}

	// У пользователя пока нет своего сообщения.
	if _, ok := important.get("admin"); ok {
		t.Fatal("get(admin) до сохранения вернул ok=true, want false")
	}

	// Сохраняем сообщение пользователю admin.
	msg, err := important.save("admin", "Прочитайте это!")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if msg.Content != "Прочитайте это!" || msg.UpdatedBy != "admin" {
		t.Fatalf("некорректное сохранённое сообщение: %+v", msg)
	}

	// Сообщения персональные: у admin есть, у other — нет.
	got, ok := important.get("admin")
	if !ok || got.Content != "Прочитайте это!" || got.UpdatedBy != "admin" {
		t.Fatalf("get(admin) = %+v, ok=%v", got, ok)
	}
	if _, ok := important.get("other"); ok {
		t.Fatal("get(other) вернул ok=true, want false (у other своего сообщения нет)")
	}

	// Сохранение другому пользователю не задевает первое.
	if _, err := important.save("other", "Моё личное"); err != nil {
		t.Fatalf("save(other): %v", err)
	}
	if got, _ := important.get("admin"); got.Content != "Прочитайте это!" {
		t.Fatalf("после save(other) сообщение admin изменилось: %q", got.Content)
	}

	// До отметки — сообщение ещё не прочитано.
	if important.seenToday("admin") {
		t.Fatal("seenToday = true до markSeen, want false")
	}

	// После отметки — прочитано (показывается раз в сутки).
	if err := important.markSeen("admin"); err != nil {
		t.Fatalf("markSeen: %v", err)
	}
	if !important.seenToday("admin") {
		t.Fatal("seenToday = false после markSeen, want true")
	}

	// Повторная отметка в тот же день не ломает состояние.
	if err := important.markSeen("admin"); err != nil {
		t.Fatalf("markSeen (повторно): %v", err)
	}
	if !important.seenToday("admin") {
		t.Fatal("seenToday = false после повторной markSeen, want true")
	}

	// Другой пользователь ещё не читал.
	if important.seenToday("other") {
		t.Fatal("seenToday(other) = true, want false")
	}
}

// В тестовом/локальном окружении (без GIN_MODE=release) сообщение
// не должно показываться — это production-only функциональность.
func TestImportantEnabledOnlyOnProduction(t *testing.T) {
	// Сбрасываем переменную, чтобы тест не зависел от окружения машины.
	t.Setenv("GIN_MODE", "")
	if importantEnabled() {
		t.Fatal("importantEnabled() = true в не-production окружении, want false")
	}

	t.Setenv("GIN_MODE", "release")
	if !importantEnabled() {
		t.Fatal("importantEnabled() = false при GIN_MODE=release, want true")
	}
}
