package tools

import (
	"bytes"
	"io/ioutil"

	"github.com/andybalholm/brotli"
)

func Brotil(data []byte) ([]byte, error) {
	if data == nil || len(data) == 0 {
		return []byte{}, nil
	}

	buffer := bytes.Buffer{}

	br := brotli.NewWriter(&buffer)
	defer br.Close()

	if err := br.Flush(); err != nil {
		return nil, err
	}

	if _, err := br.Write(data); err != nil {
		return nil, err
	}

	if err := br.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func UnBrotil(data []byte) ([]byte, error) {
	if data == nil || len(data) == 0 {
		return []byte{}, nil
	}
	reader := brotli.NewReader(bytes.NewReader(data))

	return ioutil.ReadAll(reader)
}
