package fizzbuzz

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestFizzBuzz(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "1"},
		{5, "1 2 Fizz 4 Buzz"},
		{15, "1 2 Fizz 4 Buzz Fizz 7 8 Fizz Buzz 11 Fizz 13 14 FizzBuzz"},
	}
	for _, c := range cases {
		if got := FizzBuzz(c.n); got != c.want {
			t.Errorf("FizzBuzz(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
