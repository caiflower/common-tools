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
	"fmt"
	"testing"
)

func TestPkcs7(t *testing.T) {
	s := []byte("test")
	for i := 1; i < 255; i++ {
		padding := pkcs7Padding(s, i)
		u, err := pkcs7UnPadding(padding)
		if err != nil {
			panic(err)
		}
		if string(u) != string(s) {
			panic("testPkcs7 failed.")
		}
	}
}

func TestCrypto(t *testing.T) {
	str := "test"
	encrypt, err := AesEncrypt([]byte(str), DefaultAesKey)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesEncrypt failed. err: %v", err))
	}
	decrypt, err := AesDecrypt(encrypt, DefaultAesKey)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesDecrypt failed. err: %v", err))
	}
	if string(decrypt) != str {
		panic("testCrypto failed.")
	}

	base64Encrypt, err := AesEncryptBase64(str)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesEncrypt failed. err: %v", err))
	}

	decrypt1, err := AesDecryptBase64(base64Encrypt)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesDecrypt failed. err: %v", err))
	}
	if decrypt1 != str {
		panic("testCrypto failed.")
	}

	rawBase64Encrypt, err := AesEncryptRawBase64(str)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesEncrypt failed. err: %v", err))
	}

	decrypt2, err := AesDecryptRawBase64(rawBase64Encrypt)
	if err != nil {
		panic(fmt.Sprintf("testCrypto aesDecrypt failed. err: %v", err))
	}
	if decrypt2 != str {
		panic("testCrypto failed.")
	}
}
