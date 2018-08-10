package rpcydoon
import "sync"
type RPCYDoonClientSharedInstance struct {
	*RPCYDoonClient
}
var instance *RPCYDoonClientSharedInstance
var once sync.Once
func GetInstance() *RPCYDoonClientSharedInstance {
	once.Do(func() {
		instance = &RPCYDoonClientSharedInstance{}
	})
	return instance
}
func (self *RPCYDoonClientSharedInstance) Init() {
	self.RPCYDoonClient = NewRPCYDoonClient()
}
