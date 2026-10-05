package pkgconst

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestGreeting(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"имя", "Ада", "Привет, Ада"},
		{"мир", "мир", "Привет, мир"},
		{"пустая строка", "", "Привет, "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Greeting(tc.in); got != tc.want {
				t.Errorf("Greeting(%q) = %q, ожидалось %q", tc.in, got, tc.want)
			}
		})
	}
}
