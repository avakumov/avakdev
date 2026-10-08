package tcphello

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestServeHello(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	go ServeHello(ln, "добро пожаловать")

	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatalf("подключение %d: %v", i, err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

		data, err := io.ReadAll(conn)
		conn.Close()
		if err != nil {
			t.Fatalf("чтение %d: %v", i, err)
		}
		if got := strings.TrimRight(string(data), "\r\n"); got != "добро пожаловать" {
			t.Errorf("приветствие %d = %q, ожидалось %q", i, got, "добро пожаловать")
		}
	}
}
