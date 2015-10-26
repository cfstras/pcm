package math

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

func AbsI(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
