package factorial

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestFactorial(t *testing.T) {
	cases := map[int]int{0: 1, 1: 1, 5: 120, 7: 5040}
	for n, want := range cases {
		if got := Factorial(n); got != want {
			t.Errorf("Factorial(%d) = %d, want %d", n, got, want)
		}
	}
}
