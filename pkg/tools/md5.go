package tools

import (
	"crypto/md5"
	"encoding/hex"
)

func MD5(str string) string {
	sha := md5.New()
	sha.Write([]byte(str))
	cipherStr := sha.Sum(nil)
	return hex.EncodeToString(cipherStr)
}
