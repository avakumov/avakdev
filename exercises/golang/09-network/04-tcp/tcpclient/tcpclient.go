// Пакет tcpclient — задание «TCP-клиент».
//
// Реализуйте SendLine так, чтобы прошли тесты (см. tcpclient_test.go).
// Тест поднимает собственный сервер на 127.0.0.1:0, интернет не нужен.
package tcpclient

// SendLine подключается по TCP к адресу addr, отправляет строку line
// (с переводом строки) и возвращает ответ сервера без завершающего перевода
// строки. Ошибку подключения или обмена возвращает как есть.
func SendLine(addr, line string) (string, error) {
	// TODO: подключитесь через net.Dial, отправьте line, прочитайте ответ
	// (например, bufio.NewReader(conn).ReadString('\n')) и уберите перевод
	// строки через strings.TrimRight.
	return "", nil
}
