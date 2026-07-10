package bench

import "sort"

func Median(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]int64(nil), values...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	m := len(v) / 2
	if len(v)%2 == 1 {
		return float64(v[m])
	}
	return float64(v[m-1]+v[m]) / 2
}

func MinMax(values []int64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	min, max := values[0], values[0]
	for _, n := range values[1:] {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	return float64(min), float64(max)
}
