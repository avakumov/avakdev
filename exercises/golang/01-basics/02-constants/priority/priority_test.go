package priority

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		name string
		p    int
		want string
	}{
		{"низкий", PriorityLow, "Низкий"},
		{"средний", PriorityMedium, "Средний"},
		{"высокий", PriorityHigh, "Высокий"},
		{"неизвестный", 42, ""},
		{"отрицательный", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriorityLabel(tt.p); got != tt.want {
				t.Errorf("PriorityLabel(%d) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}
