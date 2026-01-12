/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tools

import (
	"strconv"
)

// ToInt converts a string to int
func ToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// ToUint64 converts a string to uint64
func ToUint64(s string) uint64 {
	u, _ := strconv.ParseUint(s, 10, 64)
	return u
}

// ToFloat64 converts a string to float64
func ToFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}