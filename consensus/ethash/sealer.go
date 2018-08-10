package ethash
import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/log"
	"errors"
	"time"
)
func (c *Ethash) Authorize(signer common.Address, signFn SignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()
	
}
var (
	errUnauthorized = errors.New("unauthorized")
)
func (ethash *Ethash) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		header := block.Header()
		header.Nonce, header.MixDigest = types.BlockNonce{}, common.Hash{}
		return block.WithSeal(header), nil
	}
	if ethash.shared != nil {
		return ethash.shared.Seal(chain, block, stop)
	}
	abort := make(chan struct{})
	found := make(chan *types.Block)
	ethash.lock.Lock()
	threads := ethash.threads
	if ethash.rand == nil {
		seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			ethash.lock.Unlock()
			return nil, err
		}
		ethash.rand = rand.New(rand.NewSource(seed.Int64()))
	}
	ethash.lock.Unlock()
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if threads < 0 {
		threads = 0 
	}
	var pend sync.WaitGroup
	
	go ethash.mine_pos(chain, block, abort, found)
	var result *types.Block
	select {
	case <-stop:
		close(abort)
	case result = <-found:
		close(abort)
	case <-ethash.update:
		close(abort)
		pend.Wait()
		return ethash.Seal(chain, block, stop)
	}
	pend.Wait()
	return result, nil
}
func (ethash *Ethash) mine_pos(chain consensus.ChainReader, block *types.Block, abort chan struct{}, found chan *types.Block) {
	var (
		header  = block.Header()
	)
	logger := log.New("miner:pos")
	logger.Trace("Pos block.")
	producers := types.Producers{}
	header = types.CopyHeader(header)
	producers = block.Producers()
	if producers == nil || len(producers) <= 0 {
		logger.Error("producers nil error!", "number", header.Number.Uint64())
		return
	}
	if len(producers) != common.LEADER_LIMIT {
		logger.Error("len(producers) not 303")
		return
	}
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64() - 1)
	parent_slot := common.GetCurrentSlotByBigInt(parent.Time)
search:
	for {
		select {
		case <-abort:
			logger.Trace("Pos block.")
			break search
		default:
			cstart := time.Now()
			header.Time = big.NewInt(cstart.Unix())
			header.Difficulty = ethash.CalcDifficulty(chain, header, block.Transactions())
			slot := common.GetCurrentSlotByBigInt(header.Time)
			if slot <= parent_slot {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			leader_num := slot % int64(len(producers))
			leader := producers[leader_num]
			if leader.Empty() || leader.Addr != block.Coinbase() {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			select {
			case found <- block.WithSeal(header):
				logger.Trace("Ethash nonce found and reported ","slot", slot)
			case <-abort:
				logger.Trace("Ethash nonce found but discarded ","slot", slot)
			}
			break search
		}
	}
}
