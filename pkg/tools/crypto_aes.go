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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"

	"github.com/caiflower/common-tools/global/env"
)

var DefaultAesKey = []byte("01234567890123456789012345678901")

// https://blog.csdn.net/weixin_45264425/article/details/127096145
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	paddingLen := blockSize - len(ciphertext)%blockSize
	return append(ciphertext, bytes.Repeat([]byte{byte(paddingLen)}, paddingLen)...)
}

func pkcs7UnPadding(origData []byte) ([]byte, error) {
	if origData == nil {
		return nil, errors.New("wrong decrypt data")
	}
	paddingLen := int(origData[len(origData)-1])
	return origData[:len(origData)-paddingLen], nil
}

func AesEncrypt(origData []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	paddingData := pkcs7Padding(origData, blockSize)
	crypto := make([]byte, len(paddingData))
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	blockMode.CryptBlocks(crypto, paddingData)
	return crypto, nil
}

func AesDecrypt(data []byte, key []byte) ([]byte, error) {
	//创建实例
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	//获取块的大小
	blockSize := block.BlockSize()
	//使用cbc
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	//初始化解密数据接收切片
	crypto := make([]byte, len(data))
	//执行解密
	blockMode.CryptBlocks(crypto, data)
	//去除填充
	crypto, err = pkcs7UnPadding(crypto)
	if err != nil {
		return nil, err
	}
	return crypto, nil
}

// AesDecryptBase64 AesDecrypt + base64
func AesDecryptBase64(data string) (string, error) {
	key := DefaultAesKey
	if env.AESKey != "" {
		key = []byte(env.AESKey)
	}
	return AesDecryptBase64WithKey(data, key)
}

func AesDecryptBase64WithKey(data string, key []byte) (string, error) {
	decoding, err := Base64Decoding(data)
	if err != nil {
		return "", err
	}

	encrypt, err := AesDecrypt([]byte(decoding), key)
	if err != nil {
		return "", err
	}
	return string(encrypt), nil
}

// AesEncryptBase64 AesEncrypt + base64
func AesEncryptBase64(data string) (string, error) {
	key := DefaultAesKey
	if env.AESKey != "" {
		key = []byte(env.AESKey)
	}
	return AesEncryptBase64WithKey(data, key)
}

func AesEncryptBase64WithKey(data string, key []byte) (string, error) {
	encrypt, err := AesEncrypt([]byte(data), key)
	if err != nil {
		return "", err
	}
	return Base64Encoding(string(encrypt)), nil
}

// AesDecryptRawBase64 AesDecrypt + base64
func AesDecryptRawBase64(data string) (string, error) {
	key := DefaultAesKey
	if env.AESKey != "" {
		key = []byte(env.AESKey)
	}
	return AesDecryptRawBase64WithKey(data, key)
}

func AesDecryptRawBase64WithKey(data string, key []byte) (string, error) {
	decoding, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	encrypt, err := AesDecrypt(decoding, key)
	if err != nil {
		return "", err
	}
	return string(encrypt), nil
}

// AesEncryptRawBase64 AesEncrypt + base64
func AesEncryptRawBase64(data string) (string, error) {
	key := DefaultAesKey
	if env.AESKey != "" {
		key = []byte(env.AESKey)
	}
	return AesEncryptRawBase64WithKey(data, key)
}

func AesEncryptRawBase64WithKey(data string, key []byte) (string, error) {
	encrypt, err := AesEncrypt([]byte(data), key)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(encrypt), nil
}
