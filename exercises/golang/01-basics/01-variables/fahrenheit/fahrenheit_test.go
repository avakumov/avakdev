package fahrenheit

import (
	"math"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestCelsiusToFahrenheit(t *testing.T) {
	tests := []struct {
		name string
		c    float64
		want float64
	}{
		{"замерзание", 0, 32},
		{"кипение", 100, 212},
		{"пересечение шкал", -40, -40},
		{"тело человека", 37, 98.6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CelsiusToFahrenheit(tt.c)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("CelsiusToFahrenheit(%v) = %v, want %v", tt.c, got, tt.want)
			}
		})
	}
}
