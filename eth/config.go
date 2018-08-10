package eth
import (
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/consensus/ethash"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/eth/downloader"
	"github.com/DEL-ORG/del/eth/gasprice"
	"github.com/DEL-ORG/del/params"
)
var DefaultConfig = Config{
	SyncMode: downloader.FullSync,
	Ethash: ethash.Config{
		CacheDir:       "ethash",
		CachesInMem:    2,
		CachesOnDisk:   3,
		DatasetsInMem:  1,
		DatasetsOnDisk: 2,
	},
	NetworkId:     870112,
	LightPeers:    100,
	DatabaseCache: 768,
	TrieCache:     256,
	TrieTimeout:   5 * time.Minute,
	GasPrice:      big.NewInt(18 * params.Shannon),
	TxPool: core.DefaultTxPoolConfig,
	GPO: gasprice.Config{
		Blocks:     20,
		Percentile: 60,
	},
}
func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "windows" {
		DefaultConfig.Ethash.DatasetDir = filepath.Join(home, "AppData", "Ethash")
	} else {
		DefaultConfig.Ethash.DatasetDir = filepath.Join(home, ".ethash")
	}
}
type Config struct {
	Genesis *core.Genesis `toml:",omitempty"`
	NetworkId uint64 
	SyncMode  downloader.SyncMode
	NoPruning bool
	LightServ  int `toml:",omitempty"` 
	LightPeers int `toml:",omitempty"` 
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	TrieCache          int
	TrieTimeout        time.Duration
	Etherbase    common.Address `toml:",omitempty"`
	MinerThreads int            `toml:",omitempty"`
	ExtraData    []byte         `toml:",omitempty"`
	GasPrice     *big.Int
	Ethash ethash.Config
	TxPool core.TxPoolConfig
	GPO gasprice.Config
	EnablePreimageRecording bool
	DocRoot string `toml:"-"`
}
type configMarshaling struct {
	ExtraData hexutil.Bytes
}
