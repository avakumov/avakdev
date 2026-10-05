package parity

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestIsEven(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want bool
	}{
		{"ноль чётный", 0, true},
		{"положительное чётное", 4, true},
		{"положительное нечётное", 7, false},
		{"отрицательное чётное", -6, true},
		{"отрицательное нечётное", -3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEven(tt.n); got != tt.want {
				t.Errorf("IsEven(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}
