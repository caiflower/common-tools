package http

import (
	"bytes"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

var (
	MarshalErr        = fmt.Errorf("marhsal request failed")
	NewHttpRequestErr = fmt.Errorf("new httpRequest failed")
	HttpRequestErr    = fmt.Errorf("http request failed")
	UnmarshalErr      = fmt.Errorf("unmaral response failed")
	UnGzipErr         = fmt.Errorf("ungzip failed")
)

const (
	ContentTypeJson = "application/json;charset=UTF-8"
	ContentTypeForm = "application/x-www-form-urlencoded"
)

type Config struct {
	Timeout               uint  `yaml:"timeout" default:"20"`                 //请求总的超时时间, 单位：s
	MaxIdleConns          uint  `yaml:"max_idle_conns" default:"1000"`        //最大的空闲连接数，单位：s
	MaxIdleConnsPerHost   uint  `yaml:"max_idle_conns_per_host" default:"30"` //单个url的最大空闲连接数，单位：s
	ConnectTimeout        uint  `yaml:"connect_timeout" default:"30"`         //建立连接超时时间，单位：s
	KeepAliveInterval     uint  `yaml:"keep_alive_interval" default:"30"`     //存活探测间隔时间，单位：s
	IdleConnTimeout       uint  `yaml:"idle_conn_timeout" default:"500"`      //连接的最大空闲时间，单位：s
	TLSHandshakeTimeout   uint  `yaml:"tls_handshake_timeout" default:"5"`    //执行TLS握手的超时时间，单位：s
	ExpectContinueTimeout uint  `yaml:"expect_continue_timeout"`              //写Header与写Body之间，等待服务端报头的超时时间
	ResponseHeaderTimeout uint  `yaml:"response_header_timeout"`              //响应包头的最大超时时间
	DisableRetry          bool  `yaml:"disable_retry"`                        //错误不重试
	DisablePool           bool  `yaml:"disable_pool"`                         //禁用连接池意思是只使用短连接
	Verbose               *bool `yaml:"verbose" default:"false"`              //是否打印请求日志
}

type httpClient struct {
	timeout      time.Duration
	transport    *http.Transport
	disableRetry bool
	disablePool  bool
	log          logger.ILog
	verbose      bool
	setRequestId func(requestId string, header map[string]string)
}

func NewHttpClient(config Config) HttpClient {
	// 初始化默认配置
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Second * time.Duration(config.Timeout),           //建立连接超时时间
			KeepAlive: time.Second * time.Duration(config.KeepAliveInterval), //存活探测间隔时间
		}).DialContext,
		MaxIdleConns:          int(config.MaxIdleConns),                                       //最大的空闲连接数
		MaxIdleConnsPerHost:   int(config.MaxIdleConnsPerHost),                                //单个url的最大空闲连接数.
		IdleConnTimeout:       time.Second * time.Duration(config.IdleConnTimeout),            //连接的最大空闲时间
		TLSHandshakeTimeout:   time.Second * time.Duration(config.TLSHandshakeTimeout),        //执行TLS握手的超时时间
		ExpectContinueTimeout: time.Millisecond * time.Duration(config.ExpectContinueTimeout), //写Header与写Body之间，等待服务端报头的超时时间
		ResponseHeaderTimeout: time.Millisecond * time.Duration(config.ResponseHeaderTimeout), //响应包头的最大超时时间
	}

	c := &httpClient{
		timeout:      time.Second * time.Duration(config.Timeout),
		disableRetry: config.DisableRetry,
		disablePool:  config.DisablePool,
		log:          logger.DefaultLogger(),
		verbose:      *config.Verbose,
		transport:    transport,
	}

	c.log.Info("HttpClient config: %v", tools.ToJson(config))
	return c
}

func (h *httpClient) Get(requestId, url string, params map[string][]string, response *Response, header map[string]string) error {
	if len(params) > 0 {
		url += "?"
	}

	for k, v := range params {
		for _, v1 := range v {
			url += k + "=" + v1 + "&"
		}
	}
	url = url[0 : len(url)-1]

	return h.do(http.MethodGet, requestId, url, "", nil, nil, response, header)
}

func (h *httpClient) GetJson(requestId, url string, request interface{}, response *Response, header map[string]string) error {
	return h.do(http.MethodGet, requestId, url, ContentTypeJson, request, nil, response, header)
}

func (h *httpClient) PostJson(requestId, url string, request interface{}, response *Response, header map[string]string) error {
	return h.do(http.MethodPost, requestId, url, ContentTypeJson, request, nil, response, header)
}

func (h *httpClient) PostForm(requestId, u string, form map[string]interface{}, response *Response, header map[string]string) error {
	values := url.Values{}
	for k, v := range form {
		if strVal, ok := v.(string); ok {
			values.Add(k, strVal)
		} else {
			values.Add(k, tools.ToString(v))
		}
	}

	return h.do(http.MethodPost, requestId, u, ContentTypeForm, nil, values, response, header)
}

func (h *httpClient) Put(requestId, url string, request interface{}, response *Response, header map[string]string) error {
	return h.do(http.MethodPut, requestId, url, ContentTypeJson, request, nil, response, header)
}

func (h *httpClient) Patch(requestId, url string, request interface{}, response *Response, header map[string]string) error {
	return h.do(http.MethodPatch, requestId, url, ContentTypeJson, request, nil, response, header)
}

func (h *httpClient) Delete(requestId, url string, request interface{}, response *Response, header map[string]string) error {
	return h.do(http.MethodDelete, requestId, url, ContentTypeJson, request, nil, response, header)
}

func (h *httpClient) SetRequestIdCallBack(fn func(requestId string, header map[string]string)) {
	h.setRequestId = fn
}

func (h *httpClient) do(method, requestId, url, contentType string, request interface{}, values url.Values, response *Response, header map[string]string) error {
	var requestBytes []byte
	if request != nil && contentType == ContentTypeJson {
		_bytes, err := tools.Marshal(request)
		if err != nil {
			h.log.Error("marshal request failed. Error: %s", err.Error())
			return MarshalErr
		}
		requestBytes = _bytes
	}

	// 执行配置traceId函数
	if h.setRequestId != nil {
		h.setRequestId(requestId, header)
	}

	httpRequest, err := h.createHttpRequest(method, url, contentType, requestBytes, values, header)
	if err != nil {
		h.log.Error("new httpRequest failed. Error: %s", err.Error())
		return NewHttpRequestErr
	}

	if h.verbose {
		h.log.Info("%s %s URL=%s Header=%s Request=%s", requestId, method, httpRequest.URL, tools.ToJson(httpRequest.Header), requestBytes)
	}

	start := time.Now()
	client := &http.Client{Timeout: h.timeout, Transport: h.transport}
	var remoteResponse *http.Response
	fn := func() error {
		var respErr error
		remoteResponse, respErr = client.Do(httpRequest)
		if h.verbose {
			h.log.Info("%s, %s, Elapsed: %v", requestId, url, time.Now().Sub(start))
		}
		if respErr != nil {
			h.log.Error("Http远程访问出错, error: %s", respErr.Error())
			if request != nil && httpRequest.Body != nil {
				httpRequest.Body = io.NopCloser(bytes.NewBuffer(requestBytes)) //重置body
			}
			client.Timeout = 3 * time.Second //重试请求超时时间设置短一些
		}
		return respErr
	}

	var backOff backoff.BackOff
	_backOff := backoff.NewExponentialBackOff()
	max := 2 // 重试2次，一共三次
	if h.disableRetry {
		max = 0
	}
	backOff = backoff.WithMaxRetries(_backOff, uint64(max))
	respErr := backoff.Retry(fn, backOff)
	if respErr != nil {
		return HttpRequestErr
	}

	data, err := h.parseHttpResponse(remoteResponse)
	if data != nil {
		if err = tools.Unmarshal(data, response.Data); err != nil {
			h.log.Error("unmarshal remoteResponse to response failed. Error: %s", err)
			return UnmarshalErr
		}
	}
	response.StatusCode = remoteResponse.StatusCode

	return nil
}

func (h *httpClient) createHttpRequest(method, url string, contentType string, requestBytes []byte, values url.Values, header map[string]string) (*http.Request, error) {
	var reader io.Reader
	if len(requestBytes) >= 0 {
		reader = bytes.NewReader(requestBytes)
	}

	httpRequest, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		httpRequest.Header.Set("Content-Type", contentType)
		if contentType == ContentTypeForm {
			httpRequest.Form = values
		}
	}

	for k, v := range header {
		httpRequest.Header.Set(k, v)
	}

	httpRequest.Header.Set("Accept-Encoding", "gzip, br")

	if h.disablePool {
		httpRequest.Close = true
	}

	return httpRequest, nil
}

func (h *httpClient) parseHttpResponse(remoteResponse *http.Response) ([]byte, error) {
	if remoteResponse.Body != nil {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				logger.Error("close remote response body failed. Error: %s", err.Error())
			}
		}(remoteResponse.Body)
	}

	body, err := ioutil.ReadAll(remoteResponse.Body)
	if err != nil {
		h.log.Error("read remoteResponse body failed. Error: %s", err.Error())
		return nil, UnmarshalErr
	}

	if isGzip(remoteResponse.Header) {
		body, err = tools.Gunzip(body)
		if err != nil {
			h.log.Error("unzip failed. Error: %s", err.Error())
			return nil, UnGzipErr
		}
	} else if isBr(remoteResponse.Header) {
		body, err = tools.UnBrotil(body)
		if err != nil {
			h.log.Error("unzip failed. Error: %s", err.Error())
			return nil, UnGzipErr
		}
	}

	return body, err
}

func isGzip(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Content-Encoding"), "gzip")
}

func isBr(header http.Header) bool {
	if header == nil {
		return false
	}
	return strings.Contains(header.Get("Content-Encoding"), "br")
}
