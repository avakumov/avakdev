package palindrome

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestIsPalindrome(t *testing.T) {
	cases := map[string]bool{
		"А роза упала на лапу Азора": true,
		"привет": false,
		"":       true,
		"12321":  true,
		"1a2":    false,
	}
	for in, want := range cases {
		if got := IsPalindrome(in); got != want {
			t.Errorf("IsPalindrome(%q) = %v, want %v", in, got, want)
		}
	}
}
