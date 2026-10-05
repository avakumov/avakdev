// Пакет accumulator — задание «Накопитель».
//
// Реализуйте NewAccumulator так, чтобы прошли тесты (см. accumulator_test.go).
package accumulator

// NewAccumulator возвращает замыкание, которое прибавляет аргумент к сумме
// и возвращает накопленное значение.
func NewAccumulator() func(int) int {
	return func(int) int { return 0 }
}
