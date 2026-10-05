package hms

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSecondsToHMS(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		h, m, s int
	}{
		{"ноль", 0, 0, 0, 0},
		{"только секунды", 59, 0, 0, 59},
		{"ровно минута", 60, 0, 1, 0},
		{"ровно час", 3600, 1, 0, 0},
		{"смешанное", 3723, 1, 2, 3},
		{"сутки", 86400, 24, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, m, s := SecondsToHMS(tt.total)
			if h != tt.h || m != tt.m || s != tt.s {
				t.Errorf("SecondsToHMS(%d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.total, h, m, s, tt.h, tt.m, tt.s)
			}
		})
	}
}
