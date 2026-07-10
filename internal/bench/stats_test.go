package bench

import "testing"

func TestMedian(t *testing.T) {
	if got := Median([]int64{30, 10, 20}); got != 20 {
		t.Fatalf("got %v", got)
	}
	if got := Median([]int64{40, 10, 20, 30}); got != 25 {
		t.Fatalf("got %v", got)
	}
}
