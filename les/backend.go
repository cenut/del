package les
import (
	"fmt"
	"sync"
	"time"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/bloombits"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/eth"
	"github.com/DEL-ORG/del/eth/downloader"
	"github.com/DEL-ORG/del/eth/filters"
	"github.com/DEL-ORG/del/eth/gasprice"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/event"
	"github.com/DEL-ORG/del/internal/ethapi"
	"github.com/DEL-ORG/del/light"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/node"
	"github.com/DEL-ORG/del/p2p"
	"github.com/DEL-ORG/del/p2p/discv5"
	"github.com/DEL-ORG/del/params"
	rpc "github.com/DEL-ORG/del/rpc"
)
type LightEthereum struct {
	config *eth.Config
	odr         *LesOdr
	relay       *LesTxRelay
	chainConfig *params.ChainConfig
	shutdownChan chan bool
	peers           *peerSet
	txPool          *light.TxPool
	blockchain      *light.LightChain
	protocolManager *ProtocolManager
	serverPool      *serverPool
	reqDist         *requestDistributor
	retriever       *retrieveManager
	chainDb ethdb.Database 
	bloomRequests                              chan chan *bloombits.Retrieval 
	bloomIndexer, chtIndexer, bloomTrieIndexer *core.ChainIndexer
	ApiBackend *LesApiBackend
	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	networkId     uint64
	netRPCService *ethapi.PublicNetAPI
	wg sync.WaitGroup
}
func New(ctx *node.ServiceContext, config *eth.Config) (*LightEthereum, error) {
	chainDb, err := eth.CreateDB(ctx, config, "lightchaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)
	peers := newPeerSet()
	quitSync := make(chan struct{})
	leth := &LightEthereum{
		config:           config,
		chainConfig:      chainConfig,
		chainDb:          chainDb,
		eventMux:         ctx.EventMux,
		peers:            peers,
		reqDist:          newRequestDistributor(peers, quitSync),
		accountManager:   ctx.AccountManager,
		engine:           eth.CreateConsensusEngine(ctx, &config.Ethash, chainConfig, chainDb),
		shutdownChan:     make(chan bool),
		networkId:        config.NetworkId,
		bloomRequests:    make(chan chan *bloombits.Retrieval),
		bloomIndexer:     eth.NewBloomIndexer(chainDb, light.BloomTrieFrequency),
		chtIndexer:       light.NewChtIndexer(chainDb, true),
		bloomTrieIndexer: light.NewBloomTrieIndexer(chainDb, true),
	}
	leth.relay = NewLesTxRelay(peers, leth.reqDist)
	leth.serverPool = newServerPool(chainDb, quitSync, &leth.wg)
	leth.retriever = newRetrieveManager(peers, leth.reqDist, leth.serverPool)
	leth.odr = NewLesOdr(chainDb, leth.chtIndexer, leth.bloomTrieIndexer, leth.bloomIndexer, leth.retriever)
	if leth.blockchain, err = light.NewLightChain(leth.odr, leth.chainConfig, leth.engine); err != nil {
		return nil, err
	}
	leth.bloomIndexer.Start(leth.blockchain)
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		leth.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	leth.txPool = light.NewTxPool(leth.chainConfig, leth.blockchain, leth.relay)
	if leth.protocolManager, err = NewProtocolManager(leth.chainConfig, true, ClientProtocolVersions, config.NetworkId, leth.eventMux, leth.engine, leth.peers, leth.blockchain, nil, chainDb, leth.odr, leth.relay, quitSync, &leth.wg); err != nil {
		return nil, err
	}
	leth.ApiBackend = &LesApiBackend{leth, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	leth.ApiBackend.gpo = gasprice.NewOracle(leth.ApiBackend, gpoParams)
	return leth, nil
}
func lesTopic(genesisHash common.Hash, protocolVersion uint) discv5.Topic {
	var name string
	switch protocolVersion {
	case lpv1:
		name = "LES"
	case lpv2:
		name = "LES2"
	default:
		panic(nil)
	}
	return discv5.Topic(name + "@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}
type LightDummyAPI struct{}
func (s *LightDummyAPI) Etherbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}
func (s *LightDummyAPI) Mining() bool {
	return false
}
func (s *LightEthereum) APIs() []rpc.API {
	return append(ethapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}
func (s *LightEthereum) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}
func (s *LightEthereum) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightEthereum) TxPool() *light.TxPool              { return s.txPool }
func (s *LightEthereum) Engine() consensus.Engine           { return s.engine }
func (s *LightEthereum) LesVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *LightEthereum) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *LightEthereum) EventMux() *event.TypeMux           { return s.eventMux }
func (s *LightEthereum) Protocols() []p2p.Protocol {
	return s.protocolManager.SubProtocols
}
func (s *LightEthereum) Start(srvr *p2p.Server) error {
	s.startBloomHandlers()
	log.Warn("Light client mode is an experimental feature")
	s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.networkId)
	protocolVersion := AdvertiseProtocolVersions[0]
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash(), protocolVersion))
	s.protocolManager.Start(s.config.LightPeers)
	return nil
}
func (s *LightEthereum) Stop() error {
	s.odr.Stop()
	if s.bloomIndexer != nil {
		s.bloomIndexer.Close()
	}
	if s.chtIndexer != nil {
		s.chtIndexer.Close()
	}
	if s.bloomTrieIndexer != nil {
		s.bloomTrieIndexer.Close()
	}
	s.blockchain.Stop()
	s.protocolManager.Stop()
	s.txPool.Stop()
	s.eventMux.Stop()
	time.Sleep(time.Millisecond * 200)
	s.chainDb.Close()
	close(s.shutdownChan)
	return nil
}
