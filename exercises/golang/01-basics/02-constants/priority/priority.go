// Пакет priority — задание «Метка приоритета».
//
// Уровни приоритета объявлены константами через iota. Реализуйте PriorityLabel
// так, чтобы прошли тесты (см. priority_test.go).
package priority

// Уровни приоритета.
const (
	PriorityLow = iota
	PriorityMedium
	PriorityHigh
)

// PriorityLabel возвращает метку уровня приоритета.
// Для неизвестного значения возвращает пустую строку.
func PriorityLabel(p int) string {
	// TODO: верните метку через switch по константам.
	return ""
}
