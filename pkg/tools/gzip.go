package tools

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

func Gzip(data []byte) ([]byte, error) {
	if data == nil || len(data) == 0 {
		return []byte{}, nil
	}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	defer gz.Close()
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Flush(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func Gunzip(data []byte) ([]byte, error) {
	if data == nil || len(data) == 0 {
		return []byte{}, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return ioutil.ReadAll(reader)
}
