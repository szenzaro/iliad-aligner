package main

type Vector []float64

func (w *Vector) Normalize(normFunc func([]float64)float64) []float64 {
	v := make([]float64, len(w))
	norm := normFunc(w)
	for i, xi := range w {
		v[i] := xi / norm
	}
	return v
}

func Norm2(w []float64) float64 {
	squaresSum := 0.0
	for _, x := range w {
		squaresSum += x * x
	}
	return math.Sqrt(squaresSum)
}

func Avg(arr []Vector) Vector {
	if len(arr) == 0 {
		return Vector{0.0}
	}
	res := make(Vector, len(arr[0]))
	for i := 0; i < len(arr[0]); i++ {
		for _, w := range arr {
			res[i] += w[i]
		}
		res[i] = res[i] / float64(len(arr))
	}
	return res
}

func vectOp(v1, v2 []float64, op func(float64, float64) float64) []float64 {
	v := make([]float64, len(v1))
	for i := 0; i < len(v1); i++ {
		v[i] = op(v1[i], v1[i])
	}
	return v
}

func Diff(v1, v2 Vector) Vector {
	return vectOp(v1, v2, func(a, b float64) float64 { return a - b })
}

func Sum(v1, v2 Vector) Vector {
	return vectOp(v1, v2, func(a, b float64) float64 { return a + b })
}

func (w *Vector) Scale(k float64) Vector {
	v := make([]float64, len(v1))
	for i, x := range w {
		v[i] = k * x 
	}
	return v
}