package core
import (
	"time"
	"io/ioutil"
	"net/url"
	"strings"
	"encoding/json"
	"net/http"
	"net"
	"context"
	"errors"
)
var (
	DEFAULT_TRANSPORT = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   500 * time.Millisecond,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          1000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
)
type RPCClient struct {
	*http.Client
	BaseUrl string
	default_headers map[string]string
}
func NewRPCClient(base_url string, default_headers map[string]string, transport *http.Transport) (ret *RPCClient) {
	if transport == nil {
		transport = DEFAULT_TRANSPORT
	}
	ret = &RPCClient{
		Client:&http.Client{
			Transport:transport,
		},
		BaseUrl:base_url,
		default_headers:default_headers,
	}
	return ret
}
func ParamsToQueryString(params map[string]string) string {
	var pars []string
	for key, value := range params {
		pars = append(pars, key + "=" + url.QueryEscape(value))
	}
	strParams := strings.Join(pars,"&")
	return strParams
}
func (self *RPCClient)CallWithJson(ctx context.Context,t time.Time,
	method, uri string, headers map[string]string,  body interface{}, res_json interface{}) (res_body string, res_errinfo error) {
	var _method = "GET"
	if len(method) > 0 && method[0] == 'P' {
		_method = "POST"
	}
	u := self.BaseUrl
	if len(uri) > 0 && uri[0] == '/' {
		u = self.BaseUrl + uri
	}else {
		u = self.BaseUrl + "/" + uri
	}
	var err error
	var req *http.Request
	if body != nil {
		body_values, ok := body.(url.Values)
		if ok {
			req, err = http.NewRequest(_method, u, strings.NewReader(body_values.Encode()))
			if err != nil {
				return "", err
			}
		}
		body_json, ok := body.(map[string]string)
		if ok {
			b, err := json.Marshal(body_json)
			if err != nil {
				return "", err
			}
			req, err = http.NewRequest(_method, u, strings.NewReader(string(b)))
			if err != nil {
				return "", err
			}
		}
	}else {
		req, err = http.NewRequest(_method, u, http.NoBody)
		if err != nil {
			return "", err
		}
	}
	req = req.WithContext(ctx)
	req.Header.Add("Accept-Language", "zh-cn")
	if self.default_headers != nil {
		for key, value := range self.default_headers {
			req.Header.Add(key, value)
		}
	}
	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	resp, err := self.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	byte_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	res_body = string(byte_body)
	if resp.StatusCode != 200 {
		return "", errors.New("ECERRNO_HTTP_STATUS_ERROR")
	}
	if res_json != nil {
		err = json.Unmarshal(byte_body, res_json)
		if err != nil {
			return res_body, err
		}
	}
	return res_body, nil
}
