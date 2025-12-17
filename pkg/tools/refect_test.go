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
	"testing"

	"github.com/stretchr/testify/assert"
)

type Config struct {
	B      *bool    `default:"false"`
	B1     *bool    `default:"true"`
	I1     *int     `default:"1"`
	I2     *int8    `default:"2"`
	I3     *int16   `default:"3"`
	I4     *int32   `default:"4"`
	I5     *int64   `default:"5"`
	I6     *uint    `default:"6"`
	I7     *uint8   `default:"7"`
	I8     *uint16  `default:"8"`
	I9     *uint32  `default:"9"`
	I10    *uint64  `default:"10"`
	I11    int      `default:"11"`
	I12    int8     `default:"12"`
	I13    int16    `default:"13"`
	I14    int32    `default:"14"`
	I15    int64    `default:"15"`
	I16    uint     `default:"16"`
	I17    uint32   `default:"17"`
	I18    uint32   `default:"18"`
	I19    uint64   `default:"19"`
	F1     *float32 `default:"0.1"`
	F2     *float64 `default:"0.2"`
	F3     float32  `default:"0.3"`
	F4     float64  `default:"0.4"`
	Config *Config1
}

type Config1 struct {
	B   *bool    `default:"false"`
	B1  *bool    `default:"true"`
	I1  *int     `default:"1"`
	I2  *int8    `default:"2"`
	I3  *int16   `default:"3"`
	I4  *int32   `default:"4"`
	I5  *int64   `default:"5"`
	I6  *uint    `default:"6"`
	I7  *uint8   `default:"7"`
	I8  *uint16  `default:"8"`
	I9  *uint32  `default:"9"`
	I10 *uint64  `default:"10"`
	I11 int      `default:"11"`
	I12 int8     `default:"12"`
	I13 int16    `default:"13"`
	I14 int32    `default:"14"`
	I15 int64    `default:"15"`
	I16 uint     `default:"16"`
	I17 uint32   `default:"17"`
	I18 uint32   `default:"18"`
	I19 uint64   `default:"19"`
	F1  *float32 `default:"0.1"`
	F2  *float64 `default:"0.2"`
	F3  float32  `default:"0.3"`
	F4  float64  `default:"0.4"`
}

func TestSetDefaultValueIfNil(t *testing.T) {
	config := Config{}
	b1 := true
	i1 := 1
	i2 := int8(2)
	i3 := int16(3)
	i4 := int32(4)
	i5 := int64(5)
	i6 := uint(6)
	i7 := uint8(7)
	i8 := uint16(8)
	i9 := uint32(9)
	i10 := uint64(10)
	f1 := float32(0.1)
	f2 := float64(0.2)
	cmpConfig := Config{
		B:   new(bool),
		B1:  &b1,
		I1:  &i1,
		I2:  &i2,
		I3:  &i3,
		I4:  &i4,
		I5:  &i5,
		I6:  &i6,
		I7:  &i7,
		I8:  &i8,
		I9:  &i9,
		I10: &i10,
		I11: 11,
		I12: 12,
		I13: 13,
		I14: 14,
		I15: 15,
		I16: 16,
		I17: 17,
		I18: 18,
		I19: 19,
		F1:  &f1,
		F2:  &f2,
		F3:  0.3,
		F4:  0.4,
		Config: &Config1{
			B:   new(bool),
			B1:  &b1,
			I1:  &i1,
			I2:  &i2,
			I3:  &i3,
			I4:  &i4,
			I5:  &i5,
			I6:  &i6,
			I7:  &i7,
			I8:  &i8,
			I9:  &i9,
			I10: &i10,
			I11: 11,
			I12: 12,
			I13: 13,
			I14: 14,
			I15: 15,
			I16: 16,
			I17: 17,
			I18: 18,
			I19: 19,
			F1:  &f1,
			F2:  &f2,
			F3:  0.3,
			F4:  0.4,
		},
	}
	_ = DoTagFunc(&config, []FnObj{{Fn: SetDefaultValueIfNil}})
	assert.Equal(t, config, cmpConfig)
}
