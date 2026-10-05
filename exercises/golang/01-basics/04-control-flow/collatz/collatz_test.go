package collatz

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestCollatzSteps(t *testing.T) {
	cases := map[int]int{
		1:  0,
		2:  1,
		3:  7,
		6:  8,
		27: 111,
		0:  0,
	}
	for n, want := range cases {
		if got := CollatzSteps(n); got != want {
			t.Errorf("CollatzSteps(%d) = %d, want %d", n, got, want)
		}
	}
}
