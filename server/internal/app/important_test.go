package app

import (
	"testing"
)

// Тест логики «важного» сообщения на in-memory хранилище (без БД):
// персональность сообщений и отметка прочтения «раз в сутки».
func TestImportantStore(t *testing.T) {
	important := &ImportantStore{
		messages: make(map[string]ImportantMessage),
		seen:     make(map[string]string),
	}

	// У пользователя пока нет своего сообщения.
	if _, ok := important.Get("admin"); ok {
		t.Fatal("Get(admin) до сохранения вернул ok=true, want false")
	}

	// Сохраняем сообщение пользователю admin.
	msg, err := important.Save("admin", "Прочитайте это!")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if msg.Content != "Прочитайте это!" || msg.UpdatedBy != "admin" {
		t.Fatalf("некорректное сохранённое сообщение: %+v", msg)
	}

	// Сообщения персональные: у admin есть, у other — нет.
	got, ok := important.Get("admin")
	if !ok || got.Content != "Прочитайте это!" || got.UpdatedBy != "admin" {
		t.Fatalf("Get(admin) = %+v, ok=%v", got, ok)
	}
	if _, ok := important.Get("other"); ok {
		t.Fatal("Get(other) вернул ok=true, want false (у other своего сообщения нет)")
	}

	// Сохранение другому пользователю не задевает первое.
	if _, err := important.Save("other", "Моё личное"); err != nil {
		t.Fatalf("save(other): %v", err)
	}
	if got, _ := important.Get("admin"); got.Content != "Прочитайте это!" {
		t.Fatalf("после save(other) сообщение admin изменилось: %q", got.Content)
	}

	// До отметки — сообщение ещё не прочитано.
	if important.SeenToday("admin") {
		t.Fatal("SeenToday = true до MarkSeen, want false")
	}

	// После отметки — прочитано (показывается раз в сутки).
	if err := important.MarkSeen("admin"); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	if !important.SeenToday("admin") {
		t.Fatal("SeenToday = false после MarkSeen, want true")
	}

	// Повторная отметка в тот же день не ломает состояние.
	if err := important.MarkSeen("admin"); err != nil {
		t.Fatalf("MarkSeen (повторно): %v", err)
	}
	if !important.SeenToday("admin") {
		t.Fatal("SeenToday = false после повторной MarkSeen, want true")
	}

	// Другой пользователь ещё не читал.
	if important.SeenToday("other") {
		t.Fatal("SeenToday(other) = true, want false")
	}
}

// В тестовом/локальном окружении (без APP_ENV=production) сообщение
// не должно показываться — это production-only функциональность.
func TestImportantEnabledOnlyOnProduction(t *testing.T) {
	// Сбрасываем переменную, чтобы тест не зависел от окружения машины.
	t.Setenv("APP_ENV", "")
	if ImportantEnabled() {
		t.Fatal("ImportantEnabled() = true в не-production окружении, want false")
	}

	t.Setenv("APP_ENV", "production")
	if !ImportantEnabled() {
		t.Fatal("ImportantEnabled() = false при APP_ENV=production, want true")
	}
}
