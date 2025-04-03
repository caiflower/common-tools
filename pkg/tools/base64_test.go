package tools

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestBase(t *testing.T) {
	str := "hello world"

	fmt.Println(ToBase64(str))
	fmt.Println(base64.RawStdEncoding.EncodeToString([]byte(str)))
}
