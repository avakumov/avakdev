package main

import (
	"testing"
)

// Тест логики «важного» сообщения на in-memory хранилище (без БД):
// сохранение текста и отметка прочтения «раз в сутки».
func TestImportantStore(t *testing.T) {
	important = &importantStore{
		hasDB: false,
		seen:  make(map[string]string),
	}

	if err := important.save("Прочитайте это!", "admin"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := important.getContent(); got != "Прочитайте это!" {
		t.Fatalf("content = %q, want %q", got, "Прочитайте это!")
	}
	if got := important.getUpdatedBy(); got != "admin" {
		t.Fatalf("updated_by = %q, want %q", got, "admin")
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
