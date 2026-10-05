package accumulator

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestAccumulator(t *testing.T) {
	add := NewAccumulator()
	steps := []struct {
		in   int
		want int
	}{
		{5, 5},
		{3, 8},
		{-2, 6},
	}
	for _, s := range steps {
		if got := add(s.in); got != s.want {
			t.Errorf("add(%d) = %d, want %d", s.in, got, s.want)
		}
	}
}
