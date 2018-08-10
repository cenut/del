package ethash
import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/math"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/params"
	"gopkg.in/fatih/set.v0"
	"github.com/DEL-ORG/del/log"
)
var (
	BlockReward    *big.Int = big.NewInt(5e+18) 
	FrontierBlockReward    *big.Int = big.NewInt(5e+18) 
	ByzantiumBlockReward   *big.Int = big.NewInt(3e+18) 
	maxUncles                       = 2                 
	allowedFutureBlockTime          = 35 * time.Second  
)
var (
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errFastBlockTime     = errors.New("too fast timestamp")
	errOldBlockTime     = errors.New("too old timestamp")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errNonceOutOfRange   = errors.New("nonce out of range")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
	errInvalidProducer       = errors.New("invalid producer")
	errFailedToLoadHeader        = errors.New("failed to load header")
)
func (ethash *Ethash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}
func (ethash *Ethash) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return ethash.verifyHeader(chain, header, parent, false, seal)
}
func (ethash *Ethash) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	if ethash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = ethash.verifyHeaderWorker(chain, headers, seals, index)
				done <- index
			}
		}()
	}
	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}
func (ethash *Ethash) verifyHeaderWorker(chain consensus.ChainReader, headers []*types.Header, seals []bool, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
		return nil 
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index])
}
func (ethash *Ethash) VerifyDifficulty(chain consensus.ChainReader, block *types.Block) error {
	difficulty := block.Difficulty()
	excepted := ethash.CalcDifficulty(chain, block.Header(), block.Transactions())
	if difficulty.Cmp(excepted) != 0 {
		return fmt.Errorf("Miscalculation of difficulty", "difficulty", difficulty, "expected", excepted)
	}
	return nil
}
func (ethash *Ethash) VerifyVoters(chain consensus.ChainReader, block *types.Block) error {
	voters := chain.GetVoters(block.Header())
	if voters == nil {
		return errors.New("Failed to get block.")
	}
	voters_hash := types.CalcVoterHash(block.Voters)
	expected_hash := types.CalcVoterHash(voters)
	if voters_hash != expected_hash {
		aempty := (voters == nil) || (len(voters)<=0)
		bempty := (block.Voters == nil) || (len(block.Voters)<=0)
		if aempty && bempty {
			return nil
		}
		
		log.Error("Miscalculation of voters", "voters_hash", voters_hash.Hex(), "expected_hash", expected_hash.Hex())
		return fmt.Errorf("Miscalculation of voters[%s] expected[%s]\n", voters_hash.Hex(), expected_hash.Hex())
	}
	return nil
}
func (ethash *Ethash) VerifyProducers(chain consensus.ChainReader, block *types.Block) error {
	producers, err := ethash.CalProducers(chain, block.Header())
	if err != nil {
		return err
	}
	block_hash := types.CalcProducerHash(block.Producers())
	expected_hash := types.CalcProducerHash(producers)
	if block_hash != expected_hash {
		return fmt.Errorf("Miscalculation of producers", "producers_hash", block_hash, "expected", expected_hash)
	}
	slot := common.GetCurrentSlotByBigInt(block.Time())
	producer := producers[slot % int64(len(producers))]
	if producer.Empty() || block.Header().Coinbase != producer.Addr {
		parent_header := chain.GetHeader(block.Header().ParentHash, block.Number().Uint64() - 1)
		if block.Time().Int64() - parent_header.Time.Int64() < common.MINER_TIMEOUT {
			return fmt.Errorf("producer mismatch: have %s, want %s", block.Header().Coinbase.Hex(), producer.Addr.Hex())
		} else {
			genesis_body := chain.GetGenesisBlock()
			if genesis_body == nil {
				return fmt.Errorf("Can not get genesis header.")
			}
			if block.Coinbase() != genesis_body.Header().Coinbase {
				return fmt.Errorf("producer illegal have %s, want %s", block.Coinbase().Hex(), genesis_body.Header().Coinbase.Hex())
			}
		}
	}
	return nil
}
func (ethash *Ethash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
	uncles, ancestors := set.New(), make(map[common.Hash]*types.Header)
	number, parent := block.NumberU64()-1, block.ParentHash()
	for i := 0; i < 7; i++ {
		ancestor := chain.GetBlock(parent, number)
		if ancestor == nil {
			break
		}
		ancestors[ancestor.Hash()] = ancestor.Header()
		for _, uncle := range ancestor.Uncles() {
			uncles.Add(uncle.Hash())
		}
		parent, number = ancestor.ParentHash(), number-1
	}
	ancestors[block.Hash()] = block.Header()
	uncles.Add(block.Hash())
	for _, uncle := range block.Uncles() {
		hash := uncle.Hash()
		if uncles.Has(hash) {
			return errDuplicateUncle
		}
		uncles.Add(hash)
		if ancestors[hash] != nil {
			return errUncleIsAncestor
		}
		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
			return errDanglingUncle
		}
		if err := ethash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true); err != nil {
			return err
		}
	}
	return nil
}
func (ethash *Ethash) verifyHeader(chain consensus.ChainReader, header, parent *types.Header, uncle bool, seal bool) error {
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	if uncle {
		if header.Time.Cmp(math.MaxBig256) > 0 {
			return errLargeBlockTime
		}
	} else {
		if header.Time.Cmp(big.NewInt(time.Now().Add(allowedFutureBlockTime).Unix())) > 0 {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time.Cmp(parent.Time) < 0 {
		return errZeroBlockTime
	}
	if header.Time.Cmp(parent.Time) == 0 {
		log.Error("Zero blockTime", "header", header.Hash().Hex(), "number", header.Number.Uint64())
	}
	slot := common.GetCurrentSlotByBigInt(parent.Time)
	nslot := common.GetCurrentSlotByBigInt(header.Time)
	if nslot < slot {
		return errFastBlockTime
	}
	if header.Time.Cmp(big.NewInt(common.GENESIS_TIME)) < 0 {
		return errOldBlockTime
	}
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	diff := int64(parent.GasLimit) - int64(header.GasLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parent.GasLimit / params.GasLimitBoundDivisor
	if uint64(diff) >= limit || header.GasLimit < params.MinGasLimit {
		return fmt.Errorf("invalid gas limit: have %d, want %d += %d", header.GasLimit, parent.GasLimit, limit)
	}
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
	if seal {
		if err := ethash.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	
	return nil
}
func (ethash *Ethash) CalcDifficulty(chain consensus.ChainReader, header *types.Header, txs types.Transactions) *big.Int {
	difficult := new(big.Int).Set(common.ONE_COIN)
	if txs != nil {
		for _, tx := range txs {
			message, err := tx.GetMessage()
			if err != nil || message == nil {
				continue
			}
			if message.MessageID == common.DataProtocolMessageID_VOTE {
				difficult.Add(difficult, message.Tickets.TotalAmount())
			}
		}
	}
	if header.Number.Uint64() > 0 {
		difficult.Div(difficult, header.Time)
	}
	return difficult
}
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	return calcDifficulty(time, parent)
	
}
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big3          = big.NewInt(3)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	big30         = big.NewInt(30)
	bigMinus99    = big.NewInt(-99)
	bigMinus1    = big.NewInt(-1)
	bigMinus2    = big.NewInt(-2)
	big2999999    = big.NewInt(2999999)
)
func calcDifficultyByzantium(time uint64, parent *types.Header) *big.Int {
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)
	x := new(big.Int)
	y := new(big.Int)
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big9)
	if parent.UncleHash == types.EmptyUncleHash {
		x.Sub(big1, x)
	} else {
		x.Sub(big2, x)
	}
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	fakeBlockNumber := new(big.Int)
	if parent.Number.Cmp(big2999999) >= 0 {
		fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, big2999999) 
	}
	periodCount := fakeBlockNumber
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}
func calcDifficultyHomestead(time uint64, parent *types.Header) *big.Int {
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)
	x := new(big.Int)
	y := new(big.Int)
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(big1, x)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}
func calcDifficulty(time uint64, parent *types.Header) *big.Int {
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)
	x := new(big.Int)
	y := new(big.Int)
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big9)
	x.Sub(big1, x)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}
func calcDifficultyFrontier(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)
	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)
	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = math.BigMax(diff, params.MinimumDifficulty)
	}
	return diff
}
func (ethash *Ethash) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		time.Sleep(ethash.fakeDelay)
		if ethash.fakeFail == header.Number.Uint64() {
			return errInvalidPoW
		}
		return nil
	}
	if ethash.shared != nil {
		return ethash.shared.VerifySeal(chain, header)
	}
	number := header.Number.Uint64()
	if number/epochLength >= maxEpoch {
		return errNonceOutOfRange
	}
	
	return nil
}
func (ethash *Ethash) CalProducersWithoutParent(chain consensus.ChainReader, header *types.Header) (producers types.Producers, err error) {
	vproducers := types.Producers{}
	votersMap := chain.GetVotersState(header)
	if votersMap != nil {
		vproducers = votersMap.GetProducers()
	}
	if vproducers == nil || len(vproducers) <= 0 {
		genesis_header := chain.GetHeaderByNumber(0)
		vproducers = append(vproducers, types.Producer{Addr: genesis_header.Coinbase, Vote: common.Big0})
	}
	if len(vproducers) > common.LEADER_LIMIT {
		producers = vproducers[0:common.LEADER_LIMIT]
	}else {
		l := len(vproducers)
		jmp := (common.LEADER_LIMIT - l) / l
		remain := common.LEADER_LIMIT - l*jmp - l
		count := 0
		for i := 0; i < common.LEADER_LIMIT && count < l; {
			producers = append(producers, vproducers[count])
			count++
			for j := 0; j < jmp; j++ {
				producers = append(producers, types.EmptyProducer)
			}
			i+=jmp + 1
			if remain > 0 {
				i++
				remain--
				producers = append(producers, types.EmptyProducer)
			}
		}
	}
	if len(producers) != common.LEADER_LIMIT {
		log.Crit("len(producers) error", "len", len(producers))
	}
	return producers, nil
}
func (ethash *Ethash) CalProducers(chain consensus.ChainReader, header *types.Header) (producers types.Producers, err error) {
	round := common.GetRoundNumberByBlockNumber(header.Number.Uint64())
	if header.Number.Uint64() <= 0 || round <= 1 {
		genesis_header := chain.GetHeaderByNumber(0)
		genesis_block := chain.GetBlock(genesis_header.Hash(), 0)
		return genesis_block.Producers(), nil
	}
	parent_header := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent_header == nil {
		return nil, consensus.ErrUnknownAncestor
	}
	begin_block_number := common.GetBeginBlockNumberByRoundNumber(round)
	if begin_block_number != header.Number.Uint64() {
		parent_block := chain.GetBlock(parent_header.Hash(), header.Number.Uint64() - 1)
		return parent_block.Producers(), nil
	}
	return ethash.CalProducersWithoutParent(chain, parent_header)
}
func (ethash *Ethash) Prepare(chain consensus.ChainReader, header *types.Header, txs types.Transactions) (err error) {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = ethash.CalcDifficulty(chain, header, txs)
	return nil
}
func (ethash *Ethash) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt, producers types.Producers, voters types.Voters) (*types.Block, error) {
	accumulateRewards(chain, state, header, producers, voters)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	return types.NewBlock(header, txs, uncles, receipts, producers, voters), nil
}
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)
