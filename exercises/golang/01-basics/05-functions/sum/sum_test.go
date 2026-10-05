package sum

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSum(t *testing.T) {
	if got := Sum(1, 2, 3, 4); got != 10 {
		t.Errorf("Sum(1,2,3,4) = %d, want 10", got)
	}
	if got := Sum(-5, 5); got != 0 {
		t.Errorf("Sum(-5,5) = %d, want 0", got)
	}
	if got := Sum(); got != 0 {
		t.Errorf("Sum() = %d, want 0", got)
	}
}
