package shadow

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestShadowed(t *testing.T) {
	cases := []struct {
		name string
		x    int
		want int
	}{
		{"положительное", 3, 6},
		{"единица", 1, 2},
		{"отрицательное", -2, -4},
		{"большое", 10, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shadowed(tc.x); got != tc.want {
				t.Errorf("Shadowed(%d) = %d, ожидалось %d", tc.x, got, tc.want)
			}
		})
	}
}
