package math

const (
	MaxUint = ^uint(0)
	MinUint = 0
	MaxInt  = int(MaxUint >> 1)
	MinInt  = -MaxInt - 1
)

func MaxI(v1 int, val ...int) int {
	v := v1
	for _, a := range val {
		if a > v {
			v = a
		}
	}
	return v
}

func MinI(v1 int, val ...int) int {
	v := v1
	for _, a := range val {
		if a < v {
			v = a
		}
	}
	return v
}

func MaxF(v1 float32, val ...float32) float32 {
	v := v1
	for _, a := range val {
		if a > v {
			v = a
		}
	}
	return v
}

func MinF(v1 float32, val ...float32) float32 {
	v := v1
	for _, a := range val {
		if a < v {
			v = a
		}
	}
	return v
}

func AbsI(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
