package sumdigits

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSumDigits(t *testing.T) {
	cases := map[int]int{
		0:    0,
		7:    7,
		123:  6,
		9999: 36,
		-456: 15,
	}
	for n, want := range cases {
		if got := SumDigits(n); got != want {
			t.Errorf("SumDigits(%d) = %d, want %d", n, got, want)
		}
	}
}
