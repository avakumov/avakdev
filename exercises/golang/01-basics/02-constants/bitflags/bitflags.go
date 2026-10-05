// Пакет bitflags — задание «Битовые флаги».
//
// Флаги объявлены константами через iota со сдвигом. Реализуйте HasFlag так,
// чтобы прошли тесты (см. bitflags_test.go).
package bitflags

// Флаги доступа: каждый занимает отдельный бит.
const (
	FlagRead = 1 << iota
	FlagWrite
	FlagExec
)

// HasFlag сообщает, установлен ли флаг flag в наборе flags.
func HasFlag(flags, flag int) bool {
	// TODO: проверьте нужный бит побитовым И.
	return false
}
