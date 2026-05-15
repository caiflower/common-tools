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
	"encoding/hex"
	"math/big"

	"github.com/google/uuid"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const base62Len = 22

var base62 = big.NewInt(62)

func init() {
	uuid.EnableRandPool()
}

func UUID() string {
	v4, _ := uuid.NewRandom()
	var buf [32]byte
	encodeHex(buf[:], v4)
	return string(buf[:])
}

func GenerateId(prefix string) string {
	v7, _ := uuid.NewV7()

	var num big.Int
	num.SetBytes(v7[:])

	var buf [base62Len]byte
	var mod big.Int
	for i := base62Len - 1; i >= 0; i-- {
		num.DivMod(&num, base62, &mod)
		buf[i] = base62Chars[mod.Int64()]
	}

	return prefix + "-" + string(buf[:])
}

func encodeHex(dst []byte, uuid uuid.UUID) {
	hex.Encode(dst, uuid[:4])
	hex.Encode(dst[8:12], uuid[4:6])
	hex.Encode(dst[12:16], uuid[6:8])
	hex.Encode(dst[16:20], uuid[8:10])
	hex.Encode(dst[20:], uuid[10:])
}
