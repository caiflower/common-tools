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

package protocol

import "github.com/caiflower/common-tools/web/common/nocopy"

type RequestHeader struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	disableNormalizing   bool
	connectionClose      bool
	noDefaultContentType bool

	// These two fields have been moved close to other bool fields
	// for reducing RequestHeader object size.
	cookiesCollected bool

	contentLength      int
	contentLengthBytes []byte

	method      []byte
	requestURI  []byte
	host        []byte
	contentType []byte

	userAgent []byte
	mulHeader [][]byte
	protocol  string

	h     []argsKV
	bufKV argsKV

	cookies []argsKV

	// stores an immutable copy of headers as they were received from the
	// wire.
	rawHeaders []byte
}

type ResponseHeader struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	disableNormalizing   bool
	connectionClose      bool
	noDefaultContentType bool
	noDefaultDate        bool

	statusCode         int
	contentLength      int
	contentLengthBytes []byte
	contentEncoding    []byte

	contentType []byte
	server      []byte
	mulHeader   [][]byte
	protocol    string

	h     []argsKV
	bufKV argsKV

	cookies []argsKV

	headerLength int
}

type argsKV struct {
	key     []byte
	value   []byte
	noValue bool
}

func (kv *argsKV) GetKey() []byte {
	return kv.key
}

func (kv *argsKV) GetValue() []byte {
	return kv.value
}
