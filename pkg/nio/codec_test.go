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

 package nio

import (
	"fmt"
	"testing"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type test struct {
	input interface{}
	want  interface{}
}

func (v *test) valid(coderName string) {

	codec := NewCodec(logger.DefaultLogger(), coderName)
	buffer := codec.Encode(&Msg{
		flag: 1,
		body: v.input,
	})

	msg := new(Msg)
	success := codec.Decode(msg, buffer)
	if !success {
		panic(fmt.Sprintf("valid faild"))
	}

	var marshal, marshal1 []byte
	var err error

	marshal, err = tools.ToByte(v.input)
	if err != nil {
		panic(err)
	}

	marshal1 = msg.bytes

	if string(marshal) == string(marshal1) {
		fmt.Printf("test success. %s \n", string(marshal1))
	} else {
		panic(fmt.Sprintf("valid faild. input: %s want: %s, but %s", marshal, v.want, msg.bytes))
	}
}

type testStruct struct {
	TestSlice []string
	Age       int
}

func TestGzipCodec(t *testing.T) {
	var tests = []test{
		{
			input: []string{"test", "test1", "123"},
			want:  []string{"test", "test1", "123"},
		},
		{
			input: []byte{'t', 'e', 's', 't'},
			want:  []byte{'t', 'e', 's', 't'},
		},
		{
			input: []byte{'t'},
			want:  []byte{'t'},
		},
		{
			want:  "testStr",
			input: "testStr",
		},
		{
			input: testStruct{
				TestSlice: []string{"test", "test1", "123"},
				Age:       1,
			},
			want: testStruct{
				TestSlice: []string{"test", "test1", "123"},
				Age:       1,
			},
		},
		{
			input: &testStruct{
				TestSlice: []string{"test", "test1", "123"},
				Age:       1,
			},
			want: &testStruct{
				TestSlice: []string{"test", "test1", "123"},
				Age:       1,
			},
		},
		{
			input: 10,
			want:  10,
		},
	}

	for _, v := range tests {
		v.valid("")
		v.valid("gzip")
	}
}

func TestGzipCodec1(t *testing.T) {
	// 测试粘包情况
	codec := GetZipCodec(logger.DefaultLogger())
	buffer := codec.Encode(&Msg{
		flag: 1,
		body: "test1",
	})

	buffer1 := codec.Encode(&Msg{
		flag: 2,
		body: "test2",
	})
	buffer.Write(buffer1.Bytes())

	lastMsg := new(Msg)
	for codec.Decode(lastMsg, buffer) {
		msg := *lastMsg
		fmt.Printf("flag = %d, body = %s\n", msg.flag, string(msg.bytes))
	}
}
