package les
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
	"github.com/DEL-ORG/del/light"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/rpc"
)
type LesApiBackend struct {
	eth *LightEthereum
	gpo *gasprice.Oracle
}
func (s *LesApiBackend) Get24HRewardEx(address common.Address) (*types.OutputBlockReward, error) {
	return nil, nil
}
func (s *LesApiBackend) Get24HReward(address common.Address) (*big.Int, error) {
	return common.Big0, nil
}
func (b *LesApiBackend)GetVoters(ctx context.Context, blockNr rpc.BlockNumber)(voters types.Voters, err error) {
	return
}
func (s *LesApiBackend) GetVoteFreeze(ctx context.Context, address common.Address, blockNr rpc.BlockNumber)(freeze *big.Int, err error) {
	return
}
func (b *LesApiBackend)StopAutoActive(ctx context.Context) {
	return
}
func (b *LesApiBackend)StartAutoActive(ctx context.Context, addr common.Address, password string, gasPrice *big.Int) {
	return
}
func (b *LesApiBackend)StopStressTesting(ctx context.Context) {
	return
}
func (b *LesApiBackend)StartStressTesting(ctx context.Context, gasLimit uint64, gasPrice *big.Int) {
	return
}
func (b *LesApiBackend)IsVoting() bool {
	return false
}
func (b *LesApiBackend)GetVotersState(ctx context.Context, header *types.Header) (votersMap types.VotersMap, err error) {
	return nil, nil
}
func (b *LesApiBackend)GetSuperCoinbaseReward(number uint64, address common.Address) *big.Int {
	return common.Big0
}
func (b *LesApiBackend)GetVoterReward(number uint64, address common.Address) *big.Int {
	return common.Big0
}
func (b *LesApiBackend)GetCoinbaseReward(number uint64, address common.Address) *big.Int {
	return common.Big0
}
func (b *LesApiBackend)GetProducers(ctx context.Context, blockNr rpc.BlockNumber, hidden bool)(types.Producers, error) {
	return nil, nil
}
func (b *LesApiBackend)AddVoter(address common.Address, password string, producer *common.Address, gasPrice *big.Int) {
	return
}
func (b *LesApiBackend)CleanVoter() {
	return
}
func (b *LesApiBackend) ChainConfig() *params.ChainConfig {
	return b.eth.chainConfig
}
func (b *LesApiBackend) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(b.eth.BlockChain().CurrentHeader())
}
func (b *LesApiBackend) SetHead(number uint64) {
	b.eth.protocolManager.downloader.Cancel()
	b.eth.blockchain.SetHead(number)
}
func (b *LesApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	if blockNr == rpc.LatestBlockNumber || blockNr == rpc.PendingBlockNumber {
		return b.eth.blockchain.CurrentHeader(), nil
	}
	return b.eth.blockchain.GetHeaderByNumberOdr(ctx, uint64(blockNr))
}
func (b *LesApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, err
	}
	return b.GetBlock(ctx, header.Hash())
}
func (b *LesApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	return light.NewState(ctx, header, b.eth.odr), header, nil
}
func (b *LesApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.eth.blockchain.GetBlockByHash(ctx, blockHash)
}
func (b *LesApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return light.GetBlockReceipts(ctx, b.eth.odr, blockHash, core.GetBlockNumber(b.eth.chainDb, blockHash))
}
func (b *LesApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.eth.blockchain.GetTdByHash(blockHash)
}
func (b *LesApiBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	context := core.NewEVMContext(msg, header, b.eth.blockchain, nil)
	return vm.NewEVM(context, state, b.eth.chainConfig, vmCfg), state.Error, nil
}
func (b *LesApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.eth.txPool.Add(ctx, signedTx)
}
func (b *LesApiBackend) RemoveTx(txHash common.Hash) {
	b.eth.txPool.RemoveTx(txHash)
}
func (b *LesApiBackend) GetPoolTransactions() (types.Transactions, error) {
	return b.eth.txPool.GetTransactions()
}
func (b *LesApiBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	return b.eth.txPool.GetTransaction(txHash)
}
func (b *LesApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.txPool.GetNonce(ctx, addr)
}
func (b *LesApiBackend) Stats() (pending int, queued int) {
	return b.eth.txPool.Stats(), 0
}
func (b *LesApiBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.eth.txPool.Content()
}
func (b *LesApiBackend) SubscribeTxPreEvent(ch chan<- core.TxPreEvent) event.Subscription {
	return b.eth.txPool.SubscribeTxPreEvent(ch)
}
func (b *LesApiBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.eth.blockchain.SubscribeChainEvent(ch)
}
func (b *LesApiBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.eth.blockchain.SubscribeChainHeadEvent(ch)
}
func (b *LesApiBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.eth.blockchain.SubscribeChainSideEvent(ch)
}
func (b *LesApiBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.blockchain.SubscribeLogsEvent(ch)
}
func (b *LesApiBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.eth.blockchain.SubscribeRemovedLogsEvent(ch)
}
func (b *LesApiBackend) Downloader() *downloader.Downloader {
	return b.eth.Downloader()
}
func (b *LesApiBackend) ProtocolVersion() int {
	return b.eth.LesVersion() + 10000
}
func (b *LesApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}
func (b *LesApiBackend) ChainDb() ethdb.Database {
	return b.eth.chainDb
}
func (b *LesApiBackend) EventMux() *event.TypeMux {
	return b.eth.eventMux
}
func (b *LesApiBackend) AccountManager() *accounts.Manager {
	return b.eth.accountManager
}
func (b *LesApiBackend) BloomStatus() (uint64, uint64) {
	if b.eth.bloomIndexer == nil {
		return 0, 0
	}
	sections, _, _ := b.eth.bloomIndexer.Sections()
	return light.BloomTrieFrequency, sections
}
func (b *LesApiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.eth.bloomRequests)
	}
}
