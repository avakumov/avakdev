package udpclient

import (
	"net"
	"strings"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

// TestExchange поднимает простой UDP-сервер (перевод в верхний регистр) и
// проверяет работу клиента.
func TestExchange(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.ListenPacket: %v", err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo([]byte(strings.ToUpper(string(buf[:n]))), addr)
		}
	}()

	got, err := Exchange(pc.LocalAddr().String(), "привет")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got != "ПРИВЕТ" {
		t.Errorf("Exchange = %q, ожидалось %q", got, "ПРИВЕТ")
	}
}
