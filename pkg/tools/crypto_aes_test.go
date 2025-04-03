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

}
