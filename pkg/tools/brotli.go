/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
