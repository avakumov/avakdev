package minmax

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestMinMax(t *testing.T) {
	cases := []struct {
		nums []int
		min  int
		max  int
	}{
		{[]int{3, 1, 2}, 1, 3},
		{[]int{-5, 10, 0}, -5, 10},
		{[]int{7}, 7, 7},
		{nil, 0, 0},
	}
	for _, c := range cases {
		min, max := MinMax(c.nums...)
		if min != c.min || max != c.max {
			t.Errorf("MinMax(%v) = (%d, %d), want (%d, %d)", c.nums, min, max, c.min, c.max)
		}
	}
}
