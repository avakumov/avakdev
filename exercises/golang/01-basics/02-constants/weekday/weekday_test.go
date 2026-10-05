package weekday

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestWeekdayName(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"понедельник", Monday, "Понедельник"},
		{"вторник", Tuesday, "Вторник"},
		{"среда", Wednesday, "Среда"},
		{"четверг", Thursday, "Четверг"},
		{"пятница", Friday, "Пятница"},
		{"суббота", Saturday, "Суббота"},
		{"воскресенье", Sunday, "Воскресенье"},
		{"ноль", 0, ""},
		{"слишком много", 8, ""},
		{"отрицательное", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WeekdayName(tt.n); got != tt.want {
				t.Errorf("WeekdayName(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
