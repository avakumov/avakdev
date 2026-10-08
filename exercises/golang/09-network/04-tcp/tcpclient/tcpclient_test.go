package tcpclient

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

// TestSendLine поднимает простой сервер, который отвечает текстом в верхнем
// регистре, и проверяет работу клиента.
func TestSendLine(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					fmt.Fprintln(c, strings.ToUpper(scanner.Text()))
				}
			}(conn)
		}
	}()

	got, err := SendLine(ln.Addr().String(), "привет")
	if err != nil {
		t.Fatalf("SendLine: %v", err)
	}
	if got != "ПРИВЕТ" {
		t.Errorf("SendLine = %q, ожидалось %q", got, "ПРИВЕТ")
	}
}

// TestSendLineDialError проверяет, что ошибка подключения не теряется.
func TestSendLineDialError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if _, err := SendLine(addr, "привет"); err == nil {
		t.Fatal("ожидалась ошибка при подключении к закрытому порту")
	}
}
