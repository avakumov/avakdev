// Пакет weekday — задание «Название дня недели».
//
// Дни недели объявлены константами через iota. Реализуйте WeekdayName так,
// чтобы прошли тесты (см. weekday_test.go).
package weekday

// Дни недели: Monday == 1, Tuesday == 2, …, Sunday == 7.
const (
	Monday = iota + 1
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)

// WeekdayName возвращает название дня недели по его номеру.
// Для значений вне диапазона 1..7 возвращает пустую строку.
func WeekdayName(n int) string {
	// TODO: верните название дня недели через switch по константам.
	return ""
}
