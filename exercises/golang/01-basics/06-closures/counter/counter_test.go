package counter

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestCounter(t *testing.T) {
	next := NewCounter()
	for want := 1; want <= 3; want++ {
		if got := next(); got != want {
			t.Fatalf("вызов №%d = %d, want %d", want, got, want)
		}
	}

	other := NewCounter()
	if got := other(); got != 1 {
		t.Errorf("новый счётчик = %d, want 1", got)
	}
}
