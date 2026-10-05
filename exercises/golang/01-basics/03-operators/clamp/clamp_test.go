package clamp

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestClamp(t *testing.T) {
	tests := []struct {
		name  string
		x, lo int
		hi    int
		want  int
	}{
		{"внутри диапазона", 5, 1, 10, 5},
		{"ниже границы", -3, 0, 10, 0},
		{"выше границы", 42, 0, 10, 10},
		{"равен нижней границе", 0, 0, 10, 0},
		{"равен верхней границе", 10, 0, 10, 10},
		{"диапазон из одной точки", 7, 4, 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.x, tt.lo, tt.hi); got != tt.want {
				t.Errorf("Clamp(%d, %d, %d) = %d, want %d", tt.x, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}
