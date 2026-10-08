package udpecho

import (
	"net"
	"testing"
	"time"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestServeEcho(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.ListenPacket: %v", err)
	}
	defer pc.Close()
	go ServeEcho(pc)

	conn, err := net.Dial("udp", pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	for _, msg := range []string{"привет", "udp", "ещё одна датаграмма"} {
		if _, err := conn.Write([]byte(msg)); err != nil {
			t.Fatalf("запись %q: %v", msg, err)
		}
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("чтение ответа на %q: %v", msg, err)
		}
		if got := string(buf[:n]); got != msg {
			t.Errorf("эхо = %q, ожидалось %q", got, msg)
		}
	}
}
