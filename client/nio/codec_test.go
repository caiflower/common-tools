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

func (v *test) valid() {

	codec := GetZipCodec(logger.DefaultLogger())
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

	marshal, err = tools.Marshal(v.input)
	if err != nil {
		panic(err)
	}

	marshal1, err = tools.Marshal(msg.bytes)
	if err != nil {
		panic(err)
	}

	if string(msg.bytes) == string(marshal1) {
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
		v.valid()
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
