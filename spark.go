package main

import (
	"bytes"
	"math"
)

var Steps = []rune(" ▁▂▃▄▅▆▇█")

// Line generates a sparkline string from a slice of
// float32.
// Amended from https://github.com/joliv/spark , licensed under the GPLv3.
func Sparkline(nums []float32, min, max float32) string {
	if len(nums) == 0 {
		return ""
	}
	indices := normalize(nums, min, max)
	var sparkline bytes.Buffer
	for _, index := range indices {
		sparkline.WriteRune(Steps[index])
	}
	return sparkline.String()
}

func normalize(nums []float32, min, max float32) []int {
	var indices []int
	for i, _ := range nums {
		nums[i] -= min
	}
	if max == 0 {
		// Protect against division by zero
		// This can happen if all values are the same
		max = 1
	}
	for i, _ := range nums {
		x := nums[i]
		x /= max
		x *= 8
		x = float32(math.Floor(float64(x)))
		xi := int(x)
		if xi >= len(Steps) {
			xi = len(Steps) - 1
		}
		if xi <= 0 {
			xi = 0
		}

		indices = append(indices, xi)
	}
	return indices
}
