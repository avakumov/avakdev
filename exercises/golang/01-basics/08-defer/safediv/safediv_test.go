package safediv

import (
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestSafeDiv(t *testing.T) {
	cases := []struct {
		name    string
		a, b    int
		want    int
		wantErr bool
	}{
		{"делится нацело", 6, 3, 2, false},
		{"делится с остатком", 7, 2, 3, false},
		{"отрицательные", -8, 2, -4, false},
		{"деление на ноль", 5, 0, 0, true},
		{"ноль на ноль", 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeDiv(tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("SafeDiv(%d, %d): ожидалась ошибка, но её нет", tc.a, tc.b)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeDiv(%d, %d): неожиданная ошибка: %v", tc.a, tc.b, err)
			}
			if got != tc.want {
				t.Errorf("SafeDiv(%d, %d) = %d, ожидалось %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
