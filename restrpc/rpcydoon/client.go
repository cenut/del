package rpcydoon
import (
	"time"
	"context"
	"github.com/DEL-ORG/del/core"
)
const (
	BASE_URL = "http://ydoon.com:9001"
)
type RPCYDoonClient struct {
	Client *core.RPCClient
}
func NewRPCYDoonClient() (self *RPCYDoonClient) {
	self = &RPCYDoonClient{}
	self.Client = core.NewRPCClient(BASE_URL,
		map[string]string{"User-Agent":"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36"}, nil)
	return
}
func (self *RPCYDoonClient)CallWithJson(ctx context.Context,t time.Time,
	method, url string, params map[string]string, headers map[string]string, body interface{}, res_json interface{}) (res_body string, err error) {
	if params == nil {
		params = map[string]string{}
	}
	if params != nil {
		url = url + "?" + core.ParamsToQueryString(params)
	}
	res_body, err = self.Client.CallWithJson(ctx, t, method, url, headers, body, res_json)
	return
}
