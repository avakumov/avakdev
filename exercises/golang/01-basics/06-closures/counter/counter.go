// Пакет counter — задание «Счётчик».
//
// Реализуйте NewCounter так, чтобы прошли тесты (см. counter_test.go).
package counter

// NewCounter возвращает замыкание, которое при каждом вызове увеличивает
// внутренний счётчик на 1 и возвращает новое значение (1, 2, 3, ...).
func NewCounter() func() int {
	return func() int { return 0 }
}
