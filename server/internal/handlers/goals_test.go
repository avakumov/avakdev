package handlers

import "testing"

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
