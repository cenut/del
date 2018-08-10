package eth
import (
	"context"
	"math/big"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/math"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/bloombits"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core/vm"
	"github.com/DEL-ORG/del/eth/downloader"
	"github.com/DEL-ORG/del/eth/gasprice"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/event"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/rpc"
	"github.com/DEL-ORG/del/consensus/ethash"
	"github.com/DEL-ORG/del/log"
	"github.com/pkg/errors"
)
type EthApiBackend struct {
	eth *Ethereum
	gpo *gasprice.Oracle
}
func (s *EthApiBackend) Get24HRewardEx(address common.Address) (*types.OutputBlockReward, error) {
	return s.eth.BlockChain().Get24HRewardEx(address)
}
func (s *EthApiBackend) Get24HReward(address common.Address) (*big.Int, error) {
	return s.eth.BlockChain().Get24HReward(address)
}
func (s *EthApiBackend) GetVoteFreeze(ctx context.Context, address common.Address, blockNr rpc.BlockNumber)(freeze *big.Int, err error) {
	header, err := s.HeaderByNumber(ctx, blockNr)
	if err != nil {
		return common.Big0, err
	}
	if header == nil {
		return common.Big0, errors.New("Can not load header")
	}
	return s.eth.GetVoteFreeze(address, header)
}
func (b *EthApiBackend)StopAutoActive(ctx context.Context) {
	b.eth.StopAutoActive()
	return
}
func (b *EthApiBackend)StartAutoActive(ctx context.Context, addr common.Address, password string, gasPrice *big.Int) {
	b.eth.StartAutoActive(addr, password, gasPrice)
	return
}
func (b *EthApiBackend)StopStressTesting(ctx context.Context) {
	b.eth.StopStressTesting()
	return
}
func (b *EthApiBackend)StartStressTesting(ctx context.Context, gasLimit uint64, gasPrice *big.Int) {
	b.eth.StartStressTesting(gasLimit, gasPrice)
	return
}
func (b *EthApiBackend)GetVotersState(ctx context.Context, header *types.Header) (votersMap types.VotersMap, err error) {
	votersMap = b.eth.BlockChain().GetVotersState(header)
	return votersMap, nil
}
func (b *EthApiBackend)GetVoterReward(number uint64, address common.Address) *big.Int {
	return ethash.GetVoterReward(b.eth.BlockChain(), number, address)
}
func (b *EthApiBackend)GetSuperCoinbaseReward(number uint64, address common.Address) *big.Int {
	return ethash.GetSuperCoinbaseReward(b.eth.BlockChain(), number, address)
}
func (b *EthApiBackend)GetCoinbaseReward(number uint64, address common.Address) *big.Int {
	return ethash.GetCoinbaseReward(b.eth.BlockChain(), number, address)
}
func (b *EthApiBackend)StartActive(address common.Address, password string, gasPrice *big.Int) {
	b.eth.StartAutoActive(address, password, gasPrice)
	return
}
func (b *EthApiBackend)StopActive() {
	b.eth.StopAutoActive()
	return
}
func (b *EthApiBackend)AddVoter(address common.Address, password string, producer *common.Address, gasPrice *big.Int) {
	b.eth.AddVoter(address, password, producer, gasPrice)
	return
}
func (b *EthApiBackend)CleanVoter() {
	b.eth.CleanVoter()
	return
}
func (b *EthApiBackend)IsActiving() bool {
	return b.eth.IsActiving()
}
func (b *EthApiBackend)IsVoting() bool {
	return b.eth.IsVoting()
}
func (b *EthApiBackend) ChainConfig() *params.ChainConfig {
	return b.eth.chainConfig
}
func (b *EthApiBackend) CurrentBlock() *types.Block {
	return b.eth.blockchain.CurrentBlock()
}
func (b *EthApiBackend) SetHead(number uint64) {
	b.eth.protocolManager.downloader.Cancel()
	b.eth.blockchain.SetHead(number)
}
func (b *EthApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	if blockNr == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block.Header(), nil
	}
	if blockNr == rpc.LatestBlockNumber {
		return b.eth.blockchain.CurrentBlock().Header(), nil
	}
	return b.eth.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}
func (b *EthApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	if blockNr == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block, nil
	}
	if blockNr == rpc.LatestBlockNumber {
		return b.eth.blockchain.CurrentBlock(), nil
	}
	return b.eth.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}
func (b *EthApiBackend)GetProducers(ctx context.Context, blockNr rpc.BlockNumber, hidden bool)(producers types.Producers, err error) {
	if blockNr == rpc.PendingBlockNumber {
		current_block := b.CurrentBlock()
		current_header := current_block.Header()
		producers, err = b.eth.Engine().CalProducersWithoutParent(b.eth.BlockChain(), current_header)
		if err != nil {
			return nil, err
		}
		if !hidden {
			return producers, nil
		}else {
			ret := types.Producers{}
			for _, producer := range producers {
				if producer.Empty() {
					continue
				}
				ret = append(ret, types.Producer{Addr:producer.Addr, Vote:producer.Vote})
			}
			return ret, nil
		}
	}
	block, err :=b.BlockByNumber(ctx, blockNr)
	if err != nil {
		log.Error("Can not load block", "number", blockNr)
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	if !hidden {
		return block.Producers(), nil
	}else {
		ret := types.Producers{}
		for _, producer := range block.Producers() {
			if producer.Empty() {
				continue
			}
			ret = append(ret, types.Producer{Addr:producer.Addr, Vote:producer.Vote})
		}
		return ret, nil
	}
}
func (b *EthApiBackend)GetVoters(ctx context.Context, blockNr rpc.BlockNumber)(voters types.Voters, err error) {
	block, err :=b.BlockByNumber(ctx, blockNr)
	if err != nil {
		log.Error("Can not load block", "number", blockNr)
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	return block.Voters, nil
}
func (b *EthApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.eth.miner.Pending()
		return state, block.Header(), nil
	}
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.eth.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}
func (b *EthApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.eth.blockchain.GetBlockByHash(blockHash), nil
}
func (b *EthApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return core.GetBlockReceipts(b.eth.chainDb, blockHash, core.GetBlockNumber(b.eth.chainDb, blockHash)), nil
}
func (b *EthApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.eth.blockchain.GetTdByHash(blockHash)
}
func (b *EthApiBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }
	context := core.NewEVMContext(msg, header, b.eth.BlockChain(), nil)
	return vm.NewEVM(context, state, b.eth.chainConfig, vmCfg), vmError, nil
}
func (b *EthApiBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeRemovedLogsEvent(ch)
}
func (b *EthApiBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainEvent(ch)
}
func (b *EthApiBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainHeadEvent(ch)
}
func (b *EthApiBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainSideEvent(ch)
}
func (b *EthApiBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.BlockChain().SubscribeLogsEvent(ch)
}
func (b *EthApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.eth.txPool.AddLocal(signedTx)
}
func (b *EthApiBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.eth.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}
func (b *EthApiBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.eth.txPool.Get(hash)
}
func (b *EthApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.txPool.State().GetNonce(addr), nil
}
func (b *EthApiBackend) Stats() (pending int, queued int) {
	return b.eth.txPool.Stats()
}
func (b *EthApiBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.eth.TxPool().Content()
}
func (b *EthApiBackend) SubscribeTxPreEvent(ch chan<- core.TxPreEvent) event.Subscription {
	return b.eth.TxPool().SubscribeTxPreEvent(ch)
}
func (b *EthApiBackend) Downloader() *downloader.Downloader {
	return b.eth.Downloader()
}
func (b *EthApiBackend) ProtocolVersion() int {
	return b.eth.EthVersion()
}
func (b *EthApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}
func (b *EthApiBackend) ChainDb() ethdb.Database {
	return b.eth.ChainDb()
}
func (b *EthApiBackend) EventMux() *event.TypeMux {
	return b.eth.EventMux()
}
func (b *EthApiBackend) AccountManager() *accounts.Manager {
	return b.eth.AccountManager()
}
func (b *EthApiBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.eth.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}
func (b *EthApiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.eth.bloomRequests)
	}
}
