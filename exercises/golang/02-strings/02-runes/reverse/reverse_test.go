package reverse

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestReverse(t *testing.T) {
	cases := map[string]string{
		"abc":    "cba",
		"привет": "тевирп",
		"":       "",
		"a":      "a",
	}
	for in, want := range cases {
		if got := Reverse(in); got != want {
			t.Errorf("Reverse(%q) = %q, want %q", in, got, want)
		}
	}
}
