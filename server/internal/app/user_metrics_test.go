package app

import (
	"strings"
	"testing"
)

// Тест хранилища метрик на in-memory режиме (без БД): создание, типы,
// валидация значений, «один показатель в день», приватность между
// пользователями.
func TestMetricStore(t *testing.T) {
	userMetrics, err := NewMetricStore(nil)
	if err != nil {
		t.Fatalf("NewMetricStore: %v", err)
	}

	// Создание метрик разных типов.
	weight, err := userMetrics.Create("admin", "Вес", MetricTypeFloat, "кг")
	if err != nil {
		t.Fatalf("create(Вес): %v", err)
	}
	pushUps, err := userMetrics.Create("admin", "Отжимания", MetricTypeInt, "раз")
	if err != nil {
		t.Fatalf("create(Отжимания): %v", err)
	}
	smoked, err := userMetrics.Create("admin", "Курил", MetricTypeBool, "не важно")
	if err != nil {
		t.Fatalf("create(Курил): %v", err)
	}

	// Единица измерения: сохраняется для чисел, игнорируется для «да/нет».
	if weight.Unit != "кг" || pushUps.Unit != "раз" {
		t.Fatalf("unit не сохранилась: Вес=%q Отжимания=%q", weight.Unit, pushUps.Unit)
	}
	if smoked.Unit != "" {
		t.Fatalf("unit для boolean-метрики должна быть пустой, got %q", smoked.Unit)
	}

	// Редактирование: переименование и смена единицы.
	renamed, err := userMetrics.Update("admin", weight.ID, "Вес утром", "грамм")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if renamed.Name != "Вес утром" || renamed.Unit != "грамм" {
		t.Fatalf("update вернул некорректные данные: %+v", renamed)
	}
	if got, _ := userMetrics.GetOwned("admin", weight.ID); got.Name != "Вес утром" || got.Unit != "грамм" {
		t.Fatalf("метрика не обновилась в хранилище: %+v", got)
	}
	// Значения метрики при переименовании не теряются.
	if err := userMetrics.SetValue("admin", weight.ID, "2026-08-23", "78.5"); err != nil {
		t.Fatalf("setValue после update: %v", err)
	}

	// Редактирование чужой метрики отклоняется.
	if _, err := userMetrics.Update("other", weight.ID, "Чужая", ""); err == nil {
		t.Fatal("update чужой метрики должен падать")
	}

	// Некорректные данные отклоняются.
	if _, err := userMetrics.Create("admin", "   ", MetricTypeFloat, ""); err == nil {
		t.Fatal("create с пустым названием должен падать")
	}
	if _, err := userMetrics.Create("admin", "X", "date", ""); err == nil {
		t.Fatal("create с неизвестным типом должен падать")
	}
	if _, err := userMetrics.Update("admin", pushUps.ID, "  ", ""); err == nil {
		t.Fatal("update с пустым названием должен падать")
	}

	// Валидация значений по типу.
	if err := userMetrics.SetValue("admin", weight.ID, "2026-08-23", "abc"); err == nil {
		t.Fatal("float-метрика должна принимать только числа")
	}
	if err := userMetrics.SetValue("admin", pushUps.ID, "2026-08-23", "10.5"); err == nil {
		t.Fatal("int-метрика должна принимать только целые числа")
	}
	if err := userMetrics.SetValue("admin", smoked.ID, "2026-08-23", "может быть"); err == nil {
		t.Fatal("bool-метрика должна принимать только да/нет")
	}
	if err := userMetrics.SetValue("admin", weight.ID, "2026-13-99", "70"); err == nil {
		t.Fatal("некорректная дата должна отклоняться")
	}

	// Корректные значения сохраняются нормализованными.
	if err := userMetrics.SetValue("admin", weight.ID, "2026-08-23", " 78.50 "); err != nil {
		t.Fatalf("setValue(Вес): %v", err)
	}
	if err := userMetrics.SetValue("admin", pushUps.ID, "2026-08-23", "30"); err != nil {
		t.Fatalf("setValue(Отжимания): %v", err)
	}
	if err := userMetrics.SetValue("admin", smoked.ID, "2026-08-23", "да"); err != nil {
		t.Fatalf("setValue(Курил): %v", err)
	}

	// Нормализация: дробное число без лишних нулей, да/нет -> true/false.
	if v := userMetrics.Values(weight.ID)["2026-08-23"]; v != "78.5" {
		t.Fatalf("Вес = %q, want %q", v, "78.5")
	}
	if v := userMetrics.Values(smoked.ID)["2026-08-23"]; v != "true" {
		t.Fatalf("Курил = %q, want %q", v, "true")
	}

	// За день фиксируется один показатель: повторное сохранение перезаписывает.
	if err := userMetrics.SetValue("admin", weight.ID, "2026-08-23", "77.9"); err != nil {
		t.Fatalf("setValue повторно: %v", err)
	}
	vals := userMetrics.Values(weight.ID)
	if len(vals) != 1 || vals["2026-08-23"] != "77.9" {
		t.Fatalf("за день должно быть одно значение: %v", vals)
	}

	// Приватность: другой пользователь не видит и не трогает чужие метрики.
	if defs := userMetrics.List("other"); len(defs) != 0 {
		t.Fatalf("list(other) = %d метрик, want 0", len(defs))
	}
	if err := userMetrics.SetValue("other", weight.ID, "2026-08-23", "99"); err == nil ||
		!strings.Contains(err.Error(), "не найдена") {
		t.Fatalf("setValue чужим пользователем должен падать: %v", err)
	}
	if err := userMetrics.Delete("other", weight.ID); err == nil {
		t.Fatal("delete чужим пользователем должен падать")
	}

	// Удаление метрики убирает и значения.
	if err := userMetrics.Delete("admin", smoked.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := userMetrics.GetOwned("admin", smoked.ID); ok {
		t.Fatal("метрика должна исчезнуть после delete")
	}
	if vals := userMetrics.Values(smoked.ID); len(vals) != 0 {
		t.Fatalf("значения должны удалиться вместе с метрикой: %v", vals)
	}

	// Удаление значения за день.
	if err := userMetrics.DeleteValue("admin", pushUps.ID, "2026-08-23"); err != nil {
		t.Fatalf("deleteValue: %v", err)
	}
	if vals := userMetrics.Values(pushUps.ID); len(vals) != 0 {
		t.Fatalf("значение должно удалиться: %v", vals)
	}
}
