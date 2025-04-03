package tools

import "encoding/base64"

var Base64Encoding = ToBase64

func ToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Base64Decoding(str string) (string, error) {
	if _tmp, err := base64.StdEncoding.DecodeString(str); err != nil {
		return "", err
	} else {
		return string(_tmp), nil
	}
}
