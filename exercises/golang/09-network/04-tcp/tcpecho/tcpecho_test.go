package tcpecho

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

// TestServeEchoManyLines проверяет, что несколько строк на одном соединении
// возвращаются по очереди.
func TestServeEchoManyLines(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go ServeEcho(ln)

	conn := dial(t, ln)
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for _, line := range []string{"привет", "как дела", "пока"} {
		if _, err := conn.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("запись %q: %v", line, err)
		}
		got, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("чтение ответа на %q: %v", line, err)
		}
		if trimmed := strings.TrimRight(got, "\r\n"); trimmed != line {
			t.Errorf("эхо = %q, ожидалось %q", trimmed, line)
		}
	}
}

// TestServeEchoConcurrent проверяет, что сервер обслуживает несколько клиентов
// одновременно. Все соединения открываются заранее: если сервер обрабатывает
// их по очереди (без горутины на соединение), ответы придут только для первого.
func TestServeEchoConcurrent(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go ServeEcho(ln)

	const clients = 5
	conns := make([]net.Conn, clients)
	for i := range conns {
		conns[i] = dial(t, ln)
		defer conns[i].Close()
	}

	for i, conn := range conns {
		msg := fmt.Sprintf("клиент-%d", i)
		if _, err := conn.Write([]byte(msg + "\n")); err != nil {
			t.Fatalf("клиент %d, запись: %v", i, err)
		}
		got, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			t.Fatalf("клиент %d, чтение: %v", i, err)
		}
		if trimmed := strings.TrimRight(got, "\r\n"); trimmed != msg {
			t.Errorf("клиент %d: эхо = %q, ожидалось %q", i, trimmed, msg)
		}
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	return ln
}

func dial(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	return conn
}
