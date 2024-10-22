package tools

import "math/rand"

func RandInt(v1, v2 int) int {
	return rand.Intn(v2) + v1
}
