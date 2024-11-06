package tools

import "encoding/base64"

func ToBase64(str string) string {
	encoding := base64.Encoding{}
	return encoding.EncodeToString([]byte(str))
}
