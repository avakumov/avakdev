package swap

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSwap(t *testing.T) {
	tests := []struct {
		name  string
		a, b  int
		wantA int
		wantB int
	}{
		{"разные числа", 1, 2, 2, 1},
		{"отрицательные", -3, 5, 5, -3},
		{"равные числа", 7, 7, 7, 7},
		{"нули", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotA, gotB := Swap(tt.a, tt.b)
			if gotA != tt.wantA || gotB != tt.wantB {
				t.Errorf("Swap(%d, %d) = (%d, %d), want (%d, %d)",
					tt.a, tt.b, gotA, gotB, tt.wantA, tt.wantB)
			}
		})
	}
}
