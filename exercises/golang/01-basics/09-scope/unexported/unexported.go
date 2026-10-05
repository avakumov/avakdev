// Пакет unexported — задание «Экспортируемая и неэкспортируемая функции».
//
// Реализуйте Double и double так, чтобы прошли тесты (см. unexported_test.go).
package unexported

// Double — экспортируемая функция: возвращает n * 2.
// Реализуется через неэкспортируемую функцию double.
func Double(n int) int {
	return 0
}

// double — неэкспортируемая вспомогательная функция: возвращает n * 2.
func double(n int) int {
	return 0
}
