package deferredwrite

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestWithDefer(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want int
	}{
		{"ноль", 0, 1},
		{"положительное", 5, 6},
		{"отрицательное", -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithDefer(tc.n); got != tc.want {
				t.Errorf("WithDefer(%d) = %d, ожидалось %d", tc.n, got, tc.want)
			}
		})
	}
}
