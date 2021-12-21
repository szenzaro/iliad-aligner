package vectors

import (
	"math"
	"testing"
)

func TestVectorEqual(t *testing.T) {
	tt := []struct {
		l, r Vector
		out  bool
	}{
		{l: Vector{}, r: Vector{1.0}, out: false},
		{l: Vector{1, 2, 3, 4}, r: Vector{1, 2, 3, 4}, out: true},
		{l: Vector{1}, r: Vector{1.0}, out: true},
		{l: Vector{2}, r: Vector{1}, out: false},
	}

	for _, v := range tt {
		res := Equals(v.l, v.r)
		if res != v.out {
			t.Errorf("expected %v for equal %v and %v got %v", v.out, v.l, v.r, res)
		}
	}
}

func TestVectorSum(t *testing.T) {
	tt := []struct {
		l, r Vector
		out  Vector
	}{
		{Vector{}, Vector{}, Vector{}},
		{Vector{1}, Vector{1}, Vector{2}},
		{Vector{1, 2}, Vector{3, 4}, Vector{4, 6}},
		{Vector{1, 2}, Vector{0, 0}, Vector{1, 2}},
	}
	for _, v := range tt {
		res := Sum(v.l, v.r)
		if !Equals(res, v.out) {
			t.Errorf("expected %v for sum %v and %v got %v", v.out, v.l, v.r, res)
		}
	}
}

func TestVectorDiff(t *testing.T) {
	tt := []struct {
		l, r Vector
		out  Vector
	}{
		{Vector{}, Vector{}, Vector{}},
		{Vector{1}, Vector{1}, Vector{0}},
		{Vector{4, 6}, Vector{3, 4}, Vector{1, 2}},
		{Vector{1, 2}, Vector{0, 0}, Vector{1, 2}},
	}
	for _, v := range tt {
		res := Diff(v.l, v.r)
		if !Equals(res, v.out) {
			t.Errorf("expected %v for diff %v and %v got %v", v.out, v.l, v.r, res)
		}
	}
}

func TestVectorScale(t *testing.T) {
	tt := []struct {
		v   Vector
		k   float64
		out Vector
	}{
		{Vector{}, 0, Vector{}},
		{Vector{1}, 0, Vector{0}},
		{Vector{4, 6}, 1, Vector{4, 6}},
		{Vector{1, 2}, 10, Vector{10, 20}},
	}
	for _, v := range tt {
		res := v.v.Scale(v.k)
		if !Equals(res, v.out) {
			t.Errorf("expected %v for scale %v with %v got %v", v.out, v.v, v.k, res)
		}
	}
}

func TestVectorAvg(t *testing.T) {
	tt := []struct {
		v   []Vector
		out Vector
	}{
		{[]Vector{}, Vector{}},
		{[]Vector{{1, 2, 3}, {1, 2, 3}}, Vector{1, 2, 3}},
		{[]Vector{{1, 2, 3}, {4, 5, 6}}, Vector{2.5, 3.5, 4.5}},
	}
	for _, v := range tt {
		res := Avg(v.v)
		if !Equals(res, v.out) {
			t.Errorf("expected %v for avg %v got %v", v.out, v.v, res)
		}
	}
}

func TestVectorNorm2(t *testing.T) {
	tt := []struct {
		v   Vector
		out float64
	}{
		{Vector{}, 0.0},
		{Vector{1}, 1},
		{Vector{2}, 2},
		{Vector{4, 2}, math.Sqrt(20)},
	}
	for _, v := range tt {
		res := Norm2(v.v)
		if res != v.out {
			t.Errorf("expected %v for Norm2 %v got %v", v.out, v.v, res)
		}
	}
}

func TestVectorNorm(t *testing.T) {
	tt := []struct {
		v   Vector
		out Vector
	}{
		{Vector{}, Vector{}},
		{Vector{1}, Vector{1}},
		{Vector{1, 2}, Vector{1, 2}},
	}

	fn := func([]float64) float64 { return 0 }

	for _, v := range tt {
		res := v.v.Normalize(fn)
		if !Equals(res, v.out) {
			t.Errorf("expected %v for Norm %v got %v", v.out, v, res)
		}
	}

}
