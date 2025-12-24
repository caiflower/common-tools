package config

import (
	"testing"
)

func TestNewHttp2Options(t *testing.T) {
	NewHttp2Options(*NewOptions(WithDisableOptimization(true)), WithMaxUploadBufferPerStream(111))
}
