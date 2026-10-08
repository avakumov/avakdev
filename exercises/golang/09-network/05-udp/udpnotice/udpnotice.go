// Пакет udpnotice — задание «UDP-сервер-объявление».
//
// Реализуйте ServeNotice так, чтобы прошли тесты (см. udpnotice_test.go).
// Тесты поднимают сокет на loopback-адресе 127.0.0.1:0, интернет не нужен.
package udpnotice

import "net"

// ServeNotice на каждую полученную датаграмму отвечает фиксированным текстом
// notice — содержимое запроса при этом не важно. Так работают простые
// UDP-сервисы обнаружения.
func ServeNotice(conn net.PacketConn, notice string) error {
	// TODO: в цикле читайте датаграммы через conn.ReadFrom и на каждую
	// отвечайте текстом notice через conn.WriteTo.
	return nil
}
