// Пакет sum — задание «Сумма чисел».
//
// Реализуйте Sum так, чтобы прошли тесты (см. sum_test.go).
package sum

// Sum возвращает сумму всех переданных чисел. Sum() == 0.
func Sum(nums ...int) int {
	res := 0
	for _, num := range nums {
		res = res + num
	}
	return res
}
