package increment

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestIncrement(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"положительное", 5, 6},
		{"ноль", 0, 1},
		{"отрицательное", -3, -2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			Increment(&got)
			if got != tc.want {
				t.Errorf("Increment(&%d): получено %d, ожидалось %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestIncrementNil(t *testing.T) {
	// nil-указатель не должен приводить к панике.
	Increment(nil)
}
