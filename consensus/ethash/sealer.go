package ethash

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"

	"errors"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/log"
	"time"
)

func (c *Ethash) Authorize(signer common.Address, signFn SignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	/*
		c.signer = signer
		c.signFn = signFn
	*/
}

var (
	errUnauthorized = errors.New("unauthorized")
)

func (ethash *Ethash) Seal(chain consensus.ChainReader, block *types.Block, state *state.StateDB, am *accounts.Manager, coinbaseDiff *big.Int, stop <-chan struct{}) (*types.Block, error) {
	// If we're running a fake PoW, simply return a 0 nonce immediately
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		header := block.Header()
		header.Nonce, header.MixDigest = types.BlockNonce{}, common.Hash{}
		return block.WithSeal(header), nil
	}
	// If we're running a shared PoW, delegate sealing to it
	if ethash.shared != nil {
		return ethash.shared.Seal(chain, block, state, am, coinbaseDiff, stop)
	}
	// Create a runner and the multiple search threads it directs
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
		threads = 0 // Allows disabling local mining without extra logic around local/remote
	}
	var pend sync.WaitGroup

	/*
		for i := 0; i < threads; i++ {
			pend.Add(1)
			go func(id int, nonce uint64) {
				defer pend.Done()
				ethash.mine(chain, block, id, nonce, abort, found)
			}(i, uint64(ethash.rand.Int63()))
		}
	*/
	go ethash.mine_pos(chain, block, state, am, coinbaseDiff, abort, found)
	// Wait until sealing is terminated or a nonce is found
	var result *types.Block
	select {
	case <-stop:
		// Outside abort, stop all miner threads
		close(abort)
	case result = <-found:
		// One of the threads found a block, abort all others
		close(abort)
	case <-ethash.update:
		// Thread count was changed on user request, restart
		close(abort)
		pend.Wait()
		return ethash.Seal(chain, block, state, am, coinbaseDiff, stop)
	}
	// Wait for all miners to terminate and return the block
	pend.Wait()
	return result, nil
}

//每隔5秒划分一个时间槽, 也就是每5秒出一个块

func (ethash *Ethash) mine_pos(chain consensus.ChainReader, block *types.Block, state *state.StateDB, am *accounts.Manager, coinbaseDiff *big.Int, abort chan struct{}, found chan *types.Block) {
	// Extract some data from the header
	var (
		header = block.Header()
		//		number  = header.Number.Uint64()
		//		dataset = ethash.dataset(number)
	)
	// Start generating random nonces until we abort or find a good one
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
	//coinbase改变之后会引起state改变
	reward := GetRewardByNumber(header.Number.Uint64())
	if reward == nil {
		logger.Error("Can not find reward.")
		return
	}

	valid_account := map[common.Address]accounts.Account{}
	for _, wallet := range am.Wallets() {
		for _, account := range wallet.Accounts() {
			valid_account[account.Address] = account
		}
	}

	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	parent_slot := common.GetCurrentSlotByBigInt(parent.Time)

search:
	for {
		select {
		case <-abort:
			// Mining terminated, update stats and abort
			logger.Trace("Pos block.")
			break search

		default:
			cstart := time.Now()
			header.Time = big.NewInt(cstart.Unix())
			slot := common.GetCurrentSlotByBigInt(header.Time)
			if slot <= parent_slot {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			header.Difficulty = ethash.CalcDifficulty(chain, header, block.Transactions())
			leader_num := slot % int64(len(producers))
			leader := producers[leader_num]
			account, ok := valid_account[leader.Addr]
			if leader.Empty() || !ok {
				if cstart.Unix()-parent.Time.Int64() < common.MINER_TIMEOUT {
					time.Sleep(200 * time.Millisecond)
					continue
				} else {
					account, ok = valid_account[chain.GetGenesisBlock().Coinbase()]
					if !ok {
						time.Sleep(200 * time.Millisecond)
						continue
					}
				}
			}
			//只能单独挖， 不然会有很多bug.
			/*
				if header.Coinbase != account.Address {
					time.Sleep(200 * time.Millisecond)
					continue
				}
			*/
			if header.Coinbase != account.Address {
				state.SubBalance(header.Coinbase, reward.GetBlockReward())
				state.SubBalance(header.Coinbase, coinbaseDiff)
				current_round_number := common.GetRoundNumberByBlockNumber(header.Number.Uint64())
				current_round_end_block_number := common.GetEndBlockNumberByRoundNumber(current_round_number)
				if header.Number.Uint64() == current_round_end_block_number {
					voters := block.Voters
					if voters != nil && len(voters) > 0 {
						voteReward := reward.GetVoterReward()
						var totalRank uint64 = 0
						for _, voter := range voters {
							totalRank = totalRank + voter.Rank
						}
						for _, voter := range voters {
							r := new(big.Int).Set(voteReward)
							r.Mul(r, new(big.Int).SetUint64(voter.Rank))
							r.Div(r, new(big.Int).SetUint64(totalRank))
							state.SubBalance(voter.Addr, r)
						}
					}
				}

				header.Coinbase = common.HexToAddress(account.Address.Hex())
				state.AddBalance(header.Coinbase, reward.GetBlockReward())
				state.AddBalance(header.Coinbase, coinbaseDiff)
				voters := chain.GetVoters(header)
				if header.Number.Uint64() == current_round_end_block_number {
					if voters != nil && len(voters) > 0 {
						voteReward := reward.GetVoterReward()
						var totalRank uint64 = 0
						for _, voter := range voters {
							totalRank = totalRank + voter.Rank
						}
						for _, voter := range voters {
							r := new(big.Int).Set(voteReward)
							r.Mul(r, new(big.Int).SetUint64(voter.Rank))
							r.Div(r, new(big.Int).SetUint64(totalRank))
							state.AddBalance(voter.Addr, r)
						}
					}
				}
				header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
				block.Voters = nil
				if voters == nil || len(voters) == 0 {
					header.VoterHash = types.EmptyVoterHash
				} else {
					for i := range voters {
						p := types.Voter{
							Addr: common.HexToAddress(voters[i].Addr.Hex()),
						}
						p.Vote = new(big.Int).Set(voters[i].Vote)
						p.Rank = voters[i].Rank
						block.Voters = append(block.Voters, p)
					}
					header.VoterHash = types.CalcVoterHash(block.Voters)
				}
			}
			// Seal and return a block (if still needed)
			select {
			case found <- block.WithSeal(header):
				logger.Trace("Ethash nonce found and reported ", "slot", slot)
			case <-abort:
				logger.Trace("Ethash nonce found but discarded ", "slot", slot)
			}
			break search
		}
	}
	// Datasets are unmapped in a finalizer. Ensure that the dataset stays live
	// during sealing so it's not unmapped while being read.
	//	runtime.KeepAlive(dataset)
}

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
