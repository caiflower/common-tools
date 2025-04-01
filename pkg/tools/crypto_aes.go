package tools

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
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
