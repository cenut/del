package miner
import (
	"fmt"
	"sync/atomic"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/eth/downloader"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/event"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/params"
)
type Backend interface {
	AccountManager() *accounts.Manager
	BlockChain() *core.BlockChain
	TxPool() *core.TxPool
	ChainDb() ethdb.Database
}
type Miner struct {
	mux *event.TypeMux
	worker *worker
	coinbase common.Address
	mining   int32
	eth      Backend
	engine   consensus.Engine
	canStart    int32 
	shouldStart int32 
}
func New(eth Backend, config *params.ChainConfig, mux *event.TypeMux, engine consensus.Engine) *Miner {
	miner := &Miner{
		eth:      eth,
		mux:      mux,
		engine:   engine,
		worker:   newWorker(config, engine, common.Address{}, eth, mux),
		canStart: 1,
	}
	miner.Register(NewCpuAgent(eth.BlockChain(), engine, eth.AccountManager()))
	go miner.update()
	return miner
}
func (self *Miner) update() {
	events := self.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
out:
	for ev := range events.Chan() {
		switch ev.Data.(type) {
		case downloader.StartEvent:
			atomic.StoreInt32(&self.canStart, 0)
			if self.Mining() {
				self.Stop()
				atomic.StoreInt32(&self.shouldStart, 1)
				log.Info("Mining aborted due to sync")
			}
		case downloader.DoneEvent, downloader.FailedEvent:
			shouldStart := atomic.LoadInt32(&self.shouldStart) == 1
			atomic.StoreInt32(&self.canStart, 1)
			atomic.StoreInt32(&self.shouldStart, 0)
			if shouldStart {
				self.Start(self.coinbase)
			}
			events.Unsubscribe()
			break out
		}
	}
}
func (self *Miner) Start(coinbase common.Address) {
	atomic.StoreInt32(&self.shouldStart, 1)
	self.worker.setEtherbase(coinbase)
	self.coinbase = coinbase
	if atomic.LoadInt32(&self.canStart) == 0 {
		log.Info("Network syncing, will start miner afterwards")
		return
	}
	atomic.StoreInt32(&self.mining, 1)
	log.Info("Starting mining operation", "coinbase", coinbase.Hex())
	self.worker.start()
	self.worker.commitNewWork()
}
func (self *Miner) Stop() {
	self.worker.stop()
	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.shouldStart, 0)
}
func (self *Miner) Register(agent Agent) {
	if self.Mining() {
		agent.Start()
	}
	self.worker.register(agent)
}
func (self *Miner) Unregister(agent Agent) {
	self.worker.unregister(agent)
}
func (self *Miner) Mining() bool {
	return atomic.LoadInt32(&self.mining) > 0
}
func (self *Miner) HashRate() (tot int64) {
	if pow, ok := self.engine.(consensus.PoW); ok {
		tot += int64(pow.Hashrate())
	}
	for agent := range self.worker.agents {
		if _, ok := agent.(*CpuAgent); !ok {
			tot += agent.GetHashRate()
		}
	}
	return
}
func (self *Miner) SetExtra(extra []byte) error {
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("Extra exceeds max length. %d > %v", len(extra), params.MaximumExtraDataSize)
	}
	self.worker.setExtra(extra)
	return nil
}
func (self *Miner) Pending() (*types.Block, *state.StateDB) {
	return self.worker.pending()
}
func (self *Miner) PendingBlock() *types.Block {
	return self.worker.pendingBlock()
}
func (self *Miner) SetEtherbase(addr common.Address) {
	self.coinbase = addr
	self.worker.setEtherbase(addr)
}
func (self *Miner) GetEtherbase() common.Address {
	return self.coinbase
}
