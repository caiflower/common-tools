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

package basic

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

// 编译期回归：Marshaler 必须由值类型自身实现。如果只定义在指针接收者上，
// 把值放进 interface{}（map/slice 元素、gin.H、any 参数等）后不可寻址，
// 编码器拿不到 MarshalJSON，会退化成按字段反射，输出 {} 或空元素。
var (
	_ json.Marshaler = Time{}
	_ json.Marshaler = TimeStandard{}
	_ xml.Marshaler  = Time{}
	_ xml.Marshaler  = TimeStandard{}
)

const testMarshalTimeValue = `"2030-01-02 03:04:05"`

func testMarshalTime() Time {
	// 固定用 UTC，避免测试受宿主机时区影响；MarshalJSON 输出的是对象自身时区的墙上时间。
	return Time(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
}

func testMarshalTimeStandard() TimeStandard {
	return TimeStandard(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
}

type timeXMLDTO struct {
	T Time `xml:"t"`
}

func TestTimeMarshalJSON(t *testing.T) {
	bt := testMarshalTime()

	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "value", in: bt, want: testMarshalTimeValue},
		{name: "pointer", in: &bt, want: testMarshalTimeValue},
		{name: "map value", in: map[string]any{"expires_at": bt},
			want: `{"expires_at":` + testMarshalTimeValue + `}`},
		{name: "slice element", in: []any{bt},
			want: `[` + testMarshalTimeValue + `]`},
		{name: "struct value field", in: struct{ ExpiresAt Time }{ExpiresAt: bt},
			want: `{"ExpiresAt":` + testMarshalTimeValue + `}`},
		{name: "zero value", in: Time{}, want: `""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("json.Marshal(%v) error: %v", tc.in, err)
			}
			if string(got) != tc.want {
				t.Fatalf("json.Marshal(%v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestTimeStandardMarshalJSON(t *testing.T) {
	bt := testMarshalTimeStandard()

	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "value", in: bt, want: testMarshalTimeValue},
		{name: "map value", in: map[string]any{"expires_at": bt},
			want: `{"expires_at":` + testMarshalTimeValue + `}`},
		{name: "zero value", in: TimeStandard{}, want: `""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("json.Marshal(%v) error: %v", tc.in, err)
			}
			if string(got) != tc.want {
				t.Fatalf("json.Marshal(%v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestTimeMarshalXML(t *testing.T) {
	bt := testMarshalTime()

	cases := []struct {
		name string
		in   any
	}{
		{name: "value", in: bt},
		// 值结构体字段同样不可寻址（map 类型 encoding/xml 不支持）。
		{name: "struct value field", in: timeXMLDTO{T: bt}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := xml.Marshal(tc.in)
			if err != nil {
				t.Fatalf("xml.Marshal(%v) error: %v", tc.in, err)
			}
			// 非可寻址时旧实现会退化成按字段反射，输出空元素（如 <Time></Time>）。
			if !strings.Contains(string(got), "2030-01-02 03:04:05") {
				t.Fatalf("xml.Marshal(%v) = %s, want it to contain the formatted time", tc.in, got)
			}
		})
	}
}

func TestTimeJSONRoundTrip(t *testing.T) {
	bt := testMarshalTime()

	encoded, err := json.Marshal(map[string]any{"expires_at": bt})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded struct {
		ExpiresAt Time `json:"expires_at"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ExpiresAt.String() != "2030-01-02 03:04:05" {
		t.Fatalf("round trip string = %q, want %q", decoded.ExpiresAt.String(), "2030-01-02 03:04:05")
	}
	// UnmarshalJSON 会把时间规一到 time.Local，所以只断言墙上时间不变，
	// 不断言 Location/瞬时相同，避免用例依赖宿主机时区。
	if decoded.ExpiresAt.IsZero() {
		t.Fatal("round trip time is zero")
	}
}
