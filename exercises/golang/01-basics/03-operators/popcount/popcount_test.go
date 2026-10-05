package popcount

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestPopCount(t *testing.T) {
	tests := []struct {
		name string
		n    uint
		want int
	}{
		{"ноль", 0, 0},
		{"единица", 1, 1},
		{"три бита", 7, 3},
		{"степень двойки", 8, 1},
		{"байт из единиц", 255, 8},
		{"чередование", 0b1010, 2},
		{"максимум uint8", 0b11111111, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PopCount(tt.n); got != tt.want {
				t.Errorf("PopCount(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}
