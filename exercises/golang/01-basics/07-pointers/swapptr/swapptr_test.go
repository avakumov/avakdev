package swapptr

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSwapPtr(t *testing.T) {
	cases := []struct {
		name  string
		a, b  int
		wantA int
		wantB int
	}{
		{"разные значения", 1, 2, 2, 1},
		{"с нулём", 0, -5, -5, 0},
		{"равные значения", 7, 7, 7, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := tc.a, tc.b
			SwapPtr(&a, &b)
			if a != tc.wantA || b != tc.wantB {
				t.Errorf("SwapPtr(&%d, &%d) = (%d, %d), ожидалось (%d, %d)",
					tc.a, tc.b, a, b, tc.wantA, tc.wantB)
			}
		})
	}
}
