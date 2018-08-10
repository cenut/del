package node
import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"github.com/DEL-ORG/del/p2p"
	"github.com/DEL-ORG/del/p2p/nat"
	"github.com/DEL-ORG/del/common"
)
const (
	DefaultHTTPHost = "localhost" 
	DefaultHTTPPort = 7001        
	DefaultWSHost   = "localhost" 
	DefaultWSPort   = 7002        
)
var DefaultConfig = Config{
	DataDir:     DefaultDataDir(),
	HTTPPort:    DefaultHTTPPort,
	HTTPModules: []string{"net", "web3"},
	WSPort:      DefaultWSPort,
	WSModules:   []string{"net", "web3"},
	P2P: p2p.Config{
		ListenAddr: ":8001",
		MaxPeers:   25,
		NAT:        nat.Any(),
	},
}
func DefaultDataDir() string {
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", common.ClientIdentifier)
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", common.ClientIdentifier)
		} else {
			return filepath.Join(home, "." + common.ClientIdentifier)
		}
	}
	return ""
}
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
