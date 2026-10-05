// Пакет pkgconst — задание «Пакетная константа».
//
// Реализуйте Greeting так, чтобы прошли тесты (см. pkgconst_test.go).
package pkgconst

// greetingPrefix — префикс приветствия, общий для всего пакета.
const greetingPrefix = "Привет, "

// Greeting возвращает приветствие вида "Привет, <name>",
// используя пакетную константу greetingPrefix.
func Greeting(name string) string {
	return ""
}
