package rpcydoon
import (
	"time"
	"context"
	"errors"
)
type DNode struct {
	DNode  string    `json:"dnode"`
}
type ServerListRes struct {
	Code  int    `json:"code"`
	Message  string    `json:"message"`
	ServerList []DNode`json:"server_list"`
}
func (self *RPCYDoonClient) ServerList(ctx context.Context, net string) (res *ServerListRes, err error) {
	res = &ServerListRes{}
	url := "/server_list/" + net
	_, err = self.CallWithJson(ctx, time.Now(),"GET", url, nil, nil, nil, &res)
	if err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, errors.New("ECERRNO_REMOTE_SERVER_RRETURN_ERROR")
	}
	return res, nil
}
