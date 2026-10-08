// Пакет udpecho — задание «UDP-эхо-сервер».
//
// Реализуйте ServeEcho так, чтобы прошли тесты (см. udpecho_test.go).
// Тесты поднимают сокет на loopback-адресе 127.0.0.1:0, интернет не нужен.
package udpecho

import "net"

// ServeEcho в цикле читает датаграммы из conn и отправляет каждую обратно
// отправителю. Возвращает ошибку, когда conn закрыт (ReadFrom вернёт ошибку).
func ServeEcho(conn net.PacketConn) error {
	// TODO: в цикле читайте датаграммы через conn.ReadFrom в буфер и
	// отправляйте их обратно отправителю через conn.WriteTo.
	return nil
}
