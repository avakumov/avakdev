package unexported

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestDouble(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want int
	}{
		{"положительное", 4, 8},
		{"ноль", 0, 0},
		{"отрицательное", -3, -6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Double(tc.n); got != tc.want {
				t.Errorf("Double(%d) = %d, ожидалось %d", tc.n, got, tc.want)
			}
		})
	}
}

func TestDoubleUnexported(t *testing.T) {
	// Double должна быть реализована через неэкспортируемую double,
	// поэтому проверяем и её.
	cases := []struct {
		name string
		n    int
		want int
	}{
		{"положительное", 5, 10},
		{"отрицательное", -2, -4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := double(tc.n); got != tc.want {
				t.Errorf("double(%d) = %d, ожидалось %d", tc.n, got, tc.want)
			}
		})
	}
}
