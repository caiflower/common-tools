package tools

import "encoding/base64"

func ToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
