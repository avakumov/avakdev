package deferorder

import (
	"reflect"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestDeferOrder(t *testing.T) {
	got := DeferOrder()
	want := []int{3, 2, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeferOrder() = %v, ожидалось %v", got, want)
	}
}
