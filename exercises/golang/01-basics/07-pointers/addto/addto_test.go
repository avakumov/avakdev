package addto

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestAddTo(t *testing.T) {
	cases := []struct {
		name         string
		start, delta int
		want         int
	}{
		{"положительная прибавка", 10, 5, 15},
		{"отрицательная прибавка", 10, -3, 7},
		{"нулевая прибавка", 10, 0, 10},
		{"из нуля", 0, 42, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.start
			AddTo(&got, tc.delta)
			if got != tc.want {
				t.Errorf("AddTo(&%d, %d): получено %d, ожидалось %d",
					tc.start, tc.delta, got, tc.want)
			}
		})
	}
}
