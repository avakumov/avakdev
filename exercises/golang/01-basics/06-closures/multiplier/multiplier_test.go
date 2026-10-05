package multiplier

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestMakeMultiplier(t *testing.T) {
	double := MakeMultiplier(2)
	if got := double(5); got != 10 {
		t.Errorf("MakeMultiplier(2)(5) = %d, want 10", got)
	}

	triple := MakeMultiplier(3)
	if got := triple(-2); got != -6 {
		t.Errorf("MakeMultiplier(3)(-2) = %d, want -6", got)
	}
}
