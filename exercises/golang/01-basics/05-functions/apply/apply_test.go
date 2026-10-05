package apply

import (
	"reflect"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestApply(t *testing.T) {
	got := Apply([]int{1, 2, 3}, func(x int) int { return x * x })
	want := []int{1, 4, 9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Apply([1 2 3], x*x) = %v, want %v", got, want)
	}

	if got := Apply(nil, func(x int) int { return x }); len(got) != 0 {
		t.Errorf("Apply(nil) = %v, want пустой срез", got)
	}
}
