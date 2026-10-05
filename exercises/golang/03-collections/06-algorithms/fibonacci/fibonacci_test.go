package fibonacci

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestFibonacci(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 7: 13, 10: 55}
	for n, want := range cases {
		if got := Fibonacci(n); got != want {
			t.Errorf("Fibonacci(%d) = %d, want %d", n, got, want)
		}
	}
}
