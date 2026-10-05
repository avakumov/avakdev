package bitflags

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestFlagValues(t *testing.T) {
	// Значения флагов должны быть степенями двойки.
	if FlagRead != 1 {
		t.Errorf("FlagRead = %d, want 1", FlagRead)
	}
	if FlagWrite != 2 {
		t.Errorf("FlagWrite = %d, want 2", FlagWrite)
	}
	if FlagExec != 4 {
		t.Errorf("FlagExec = %d, want 4", FlagExec)
	}
}

func TestHasFlag(t *testing.T) {
	flags := FlagRead | FlagWrite

	tests := []struct {
		name string
		flag int
		want bool
	}{
		{"установлен read", FlagRead, true},
		{"установлен write", FlagWrite, true},
		{"не установлен exec", FlagExec, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasFlag(flags, tt.flag); got != tt.want {
				t.Errorf("HasFlag(%d, %d) = %v, want %v", flags, tt.flag, got, tt.want)
			}
		})
	}
}
