package unique

import (
	"reflect"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestUnique(t *testing.T) {
	got := Unique([]int{3, 1, 3, 2, 1, 4})
	want := []int{3, 1, 2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unique([3 1 3 2 1 4]) = %v, want %v", got, want)
	}
}
