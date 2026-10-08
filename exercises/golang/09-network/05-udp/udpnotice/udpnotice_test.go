package udpnotice

import (
	"net"
	"testing"
	"time"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestServeNotice(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.ListenPacket: %v", err)
	}
	defer pc.Close()
	go ServeNotice(pc, "сервер работает")

	conn, err := net.Dial("udp", pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write([]byte("любой запрос")); err != nil {
		t.Fatalf("запись: %v", err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if got := string(buf[:n]); got != "сервер работает" {
		t.Errorf("ответ = %q, ожидалось %q", got, "сервер работает")
	}
}
