package http

import (
	"fmt"
	"testing"

	"github.com/caiflower/common-tools/pkg/tools"
)

// Event结构体表示单个事件
type Event struct {
	Event        string `json:"event"`
	Params       string `json:"params"`
	LocalTimeMs  int64  `json:"local_time_ms"`
	IsBav        int    `json:"is_bav"`
	AbSdkVersion string `json:"ab_sdk_version"`
	SessionId    string `json:"session_id"`
}

// User结构体表示用户信息
type User struct {
	UserUniqueId string `json:"user_unique_id"`
	UserId       string `json:"userId"`
	UserIsLogin  bool   `json:"user_is_login"`
	WebId        string `json:"web_id"`
}

// Header结构体表示头部信息
type Header struct {
	AppId          int    `json:"app_id"`
	OsName         string `json:"os_name"`
	OsVersion      string `json:"os_version"`
	DeviceModel    string `json:"device_model"`
	Language       string `json:"language"`
	Platform       string `json:"platform"`
	SdkVersion     string `json:"sdk_version"`
	SdkLib         string `json:"sdk_lib"`
	Timezone       int    `json:"timezone"`
	TzOffset       int    `json:"tz_offset"`
	Resolution     string `json:"resolution"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	Referrer       string `json:"referrer"`
	ReferrerHost   string `json:"referrer_host"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ScreenWidth    int    `json:"screen_width"`
	ScreenHeight   int    `json:"screen_height"`
	TracerData     string `json:"tracer_data"`
	Custom         string `json:"custom"`
}

// Data结构体表示整个JSON数据结构的顶层对象
type Data struct {
	Events    []Event `json:"events"`
	User      User    `json:"user"`
	Header    Header  `json:"header"`
	LocalTime int64   `json:"local_time"`
	Verbose   int     `json:"verbose"`
}

type Res struct {
	E  int
	Sc int
	Tc int
}

func TestHttpClient(t *testing.T) {
	config := Config{}
	enable := true
	config.Verbose = &enable
	client := NewHttpClient(config)

	jsonData := []byte(`[
    {
        "events": [
            {
                "event": "send_message",
                "params": "{\"_staging_flag\":0,\"conversation_id\":\"77323444686594\",\"current_page\":\"search_engine_google\",\"scene\":\"link_preview\",\"event_index\":1731762982443}",
                "local_time_ms": 1731762813085,
                "is_bav": 0,
                "ab_sdk_version": "9733657,9906316,9696148",
                "session_id": "53a8d549-2b14-438d-a2bb-0fe603fa25cd"
            }
        ],
        "user": {
            "user_unique_id": "416485979780064963",
            "user_id": "235743231094571",
            "user_is_login": true,
            "web_id": "7437717125680842294"
        },
        "header": {
            "app_id": 586864,
            "os_name": "mac",
            "os_version": "10_15_7",
            "device_model": "Macintosh",
            "language": "zh-CN",
            "platform": "web",
            "sdk_version": "5.1.19",
            "sdk_lib": "js",
            "timezone": 8,
            "tz_offset": -28800,
            "resolution": "2560x1440",
            "browser": "Chrome",
            "browser_version": "130.0.0.0",
            "referrer": "https://www.google.com.hk/",
            "referrer_host": "www.google.com.hk",
            "width": 2560,
            "height": 1440,
            "screen_width": 2560,
            "screen_height": 1440,
            "tracer_data": "{\"$utm_from_url\":1}",
            "custom": "{\"extension_version\":\"1.13.4\"}"
        },
        "local_time": 1731762813,
        "verbose": 1
    }
]`)

	var data []Data
	err := tools.Unmarshal(jsonData, &data)
	if err != nil {
		fmt.Println(err)
	}

	res := &Res{}
	response := Response{Data: res}
	err = client.PostJson("test", "https://mcs.zijieapi.com/list", data, &response, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", res)
}

type InnerParam struct {
	TestId  string `json:"testId" inList:"testId1"`
	TestInt []int  `json:"testInt" inList:"1,2,3,4,5" reg:"[1-3]+" between:"1,2"`
}

type Param struct {
	TestId     string
	Args       string   `json:"args" param:"args" default:"testDefault"`
	Name       string   `json:"name"`
	Name1      *string  `verf:"nilable" len:",5"`
	MyName     []string `json:"myName" inList:"myName,myName1" reg:"[0-9a-zA-Z]+"`
	TestInt    []int    `json:"testInt" inList:"1,2,3,4,5" reg:"[1-3]+" between:"1,2"`
	InnerParam *InnerParam
}

type CommonResponse struct {
	RequestID string `json:"requestId"`
	Code      *int   `json:"code,omitempty"`
	Data      Param  `json:"data,omitempty"`
}

func TestHttpClient_GetJson(t *testing.T) {
	config := Config{}
	enable := true
	config.Verbose = &enable
	client := NewHttpClient(config)

	s := "name1"
	var request = Param{
		TestId:  "mysql",
		Args:    "1",
		Name:    "2",
		Name1:   &s,
		MyName:  []string{"myName"},
		TestInt: []int{1, 2},
	}

	var res CommonResponse
	response := Response{
		Data: &res,
	}
	err := client.PostJson("test", "http://127.0.0.1:8080/v1/test/struct?Action=Test1", request, &response, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v\n", res)
}
