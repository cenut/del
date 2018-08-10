package ethash
import (
	"math/big"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/log"
)
type Reward struct {
	Number uint64
	BlockReward *big.Int
	Reward *big.Int
}
func NewReward(number uint64, blockReward, reward string) (ret *Reward){
	ret = &Reward{
		Number:number,
		BlockReward:new(big.Int),
		Reward:new(big.Int),
	}
	ret.BlockReward.UnmarshalText([]byte(blockReward))
	ret.Reward.UnmarshalText([]byte(reward))
	return ret
}
func GetRewardByNumber(number uint64)(reward *Reward) {
	for i := 0; i < len(Rewards); i++ {
		reward = Rewards[i]
		if number <= reward.Number {
			return Rewards[i]
		}
	}
	return reward
}
func (self *Reward)GetBlockReward()(blockReward *big.Int) {
	return self.BlockReward
}
func (self *Reward)GetCoinbaseReward()(coinbaseReward *big.Int) {
	coinbaseReward = new(big.Int).Set(self.Reward)
	coinbaseReward.Mul(coinbaseReward, big.NewInt(40))
	coinbaseReward.Div(coinbaseReward, big.NewInt(100))
	return coinbaseReward
}
func (self *Reward)GetSuperCoinbaseReward()(superCoinbaseReward *big.Int) {
	superCoinbaseReward = new(big.Int).Set(self.Reward)
	superCoinbaseReward.Mul(superCoinbaseReward, big.NewInt(10))
	superCoinbaseReward.Div(superCoinbaseReward, big.NewInt(100))
	return superCoinbaseReward
}
func (self *Reward)GetVoterReward()(voteReward *big.Int) {
	voteReward = new(big.Int).Set(self.Reward)
	voteReward.Mul(voteReward, big.NewInt(50))
	voteReward.Div(voteReward, big.NewInt(100))
	return voteReward
}
func GetVoterReward(chain consensus.ChainReader, number uint64, address common.Address) *big.Int{
	reward := GetRewardByNumber(number)
	if reward == nil {
		return common.Big0
	}
	header := chain.GetHeaderByNumber(number)
	if header == nil {
		return common.Big0
	}
	voters := chain.GetVoters(header)
	if voters == nil {
		return common.Big0
	}
	voter := &types.Voter{}
	voter = nil
	var totalRank uint64 = 0
	for _, v := range voters {
		if v.Addr == address {
			voter = &types.Voter{
				Addr:common.HexToAddress(v.Addr.Hex()),
				Vote:big.NewInt(0).Set(v.Vote),
				Rank:v.Rank,
			}
		}
		totalRank = totalRank + v.Rank
	}
	if voter == nil {
		return common.Big0
	}
	voteReward := reward.GetVoterReward()
	voteReward.Mul(voteReward, new(big.Int).SetUint64(voter.Rank))
	voteReward.Div(voteReward, new(big.Int).SetUint64(totalRank))
	return voteReward
}
func GetCoinbaseReward(chain consensus.ChainReader, number uint64, address common.Address) *big.Int{
	reward := GetRewardByNumber(number)
	if reward == nil {
		return common.Big0
	}
	round_number := common.GetRoundNumberByBlockNumber(number)
	block_number := common.GetBeginBlockNumberByRoundNumber(round_number)
	producer_header := chain.GetHeaderByNumber(block_number)
	if producer_header == nil {
		log.Error("Can not find header", "number", block_number)
		return common.Big0
	}
	producer_block := chain.GetBlock(producer_header.Hash(), producer_header.Number.Uint64())
	if producer_block == nil {
		log.Error("Can not find block", "number", block_number)
		return common.Big0
	}
	producers := producer_block.Producers()
	if producers == nil || len(producers) <= 0 {
		log.Error("Empty producers")
		return common.Big0
	}
	total := 0
	for i := 0; i < len(producers); i++ {
		if producers[i].Empty() {
			continue
		}
		total++
	}
	coinbaseReward := reward.GetCoinbaseReward()
	coinbaseNum := len(producers)
	for i := 0; i < coinbaseNum; i++ {
		if producers[i].Empty() {
			continue
		}
		if producers[i].Addr == address {
			r := new(big.Int).Set(coinbaseReward)
			r.Mul(r, new(big.Int).SetUint64(1))
			r.Div(r, new(big.Int).SetInt64(int64(total)))
			return r
		}
	}
	return common.Big0
}
func GetSuperCoinbaseReward(chain consensus.ChainReader, number uint64, address common.Address) *big.Int{
	reward := GetRewardByNumber(number)
	if reward == nil {
		return common.Big0
	}
	round_number := common.GetRoundNumberByBlockNumber(number)
	block_number := common.GetBeginBlockNumberByRoundNumber(round_number)
	producer_header := chain.GetHeaderByNumber(block_number)
	if producer_header == nil {
		log.Error("Can not find header", "number", block_number)
		return common.Big0
	}
	producer_block := chain.GetBlock(producer_header.Hash(), producer_header.Number.Uint64())
	if producer_block == nil {
		log.Error("Can not find block", "number", block_number)
		return common.Big0
	}
	producers := producer_block.Producers()
	if producers == nil || len(producers) <= 0 {
		log.Error("Empty producers")
		return common.Big0
	}
	if len(producers) != common.LEADER_LIMIT {
		log.Error("len(Producers) not 303", "len", len(producers))
	}
	total := 0
	for i := 0; i < len(producers); i++ {
		if producers[i].Empty() {
			continue
		}
		total++
	}
	totalSuper := total
	if totalSuper > common.SUPER_COINBASE_RANK {
		totalSuper = common.SUPER_COINBASE_RANK
	}
	superCoinbaseReward := reward.GetSuperCoinbaseReward()
	superCoinbaseNum := 0
	for i := 0; i < len(producers); i++ {
		if producers[i].Empty() {
			continue
		}
		if superCoinbaseNum >= common.SUPER_COINBASE_RANK {
			break
		}
		if producers[i].Addr == address {
			r := new(big.Int).Set(superCoinbaseReward)
			r.Mul(r, new(big.Int).SetUint64(1))
			r.Div(r, new(big.Int).SetInt64(int64(totalSuper)))
			return r
		}
		superCoinbaseNum++
	}
	return common.Big0
}
func accumulateRewards(chain consensus.ChainReader, state *state.StateDB, header *types.Header, producers types.Producers, voters types.Voters) {
	reward := GetRewardByNumber(header.Number.Uint64())
	current_round_number := common.GetRoundNumberByBlockNumber(header.Number.Uint64())
	current_round_end_block_number := common.GetEndBlockNumberByRoundNumber(current_round_number)
	if header.Number.Uint64() == current_round_end_block_number {
		current_round_begin_block_number := common.GetBeginBlockNumberByRoundNumber(current_round_number)
		total := 0
		for i := 0; i < len(producers); i++ {
			if producers[i].Empty() {
				continue
			}
			total++
		}
		totalSuper := total
		if totalSuper > common.SUPER_COINBASE_RANK {
			totalSuper = common.SUPER_COINBASE_RANK
		}
		coinbaseReward := reward.GetCoinbaseReward()
		superCoinbaseReward := reward.GetSuperCoinbaseReward()
		processVoter := func(voters types.Voters) {
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
		processCoinbase := func(producers types.Producers) {
			for i := 0; i < len(producers); i++ {
				if producers[i].Empty() {
					continue
				}
				r := new(big.Int).Set(coinbaseReward)
				r.Div(r, new(big.Int).SetInt64(int64(total)))
				state.AddBalance(producers[i].Addr, r)
			}
		}
		processSuperCoinbase := func(producers types.Producers) {
			superCoinbaseNum := 0
			for i := 0; i < len(producers); i++ {
				if producers[i].Empty() {
					continue
				}
				if superCoinbaseNum >= common.SUPER_COINBASE_RANK {
					break
				}
				r := new(big.Int).Set(superCoinbaseReward)
				r.Div(r, new(big.Int).SetInt64(int64(totalSuper)))
				state.AddBalance(producers[i].Addr, r)
				superCoinbaseNum++
			}
		}
		processVoter(voters)
		processCoinbase(producers)
		processSuperCoinbase(producers)
		for h := chain.GetHeader(header.ParentHash, header.Number.Uint64() - 1);
			h != nil && h.Number.Uint64() >= current_round_begin_block_number;
			h = chain.GetHeader(h.ParentHash, h.Number.Uint64() - 1) {
			block := chain.GetBlock(h.Hash(), h.Number.Uint64())
			if block == nil {
				log.Error("Can not load block", "hash", h.Hash())
				continue
			}
			processVoter(block.Voters)
			processCoinbase(block.Producers())
			processSuperCoinbase(block.Producers())
		}
	}
	state.AddBalance(header.Coinbase, reward.GetBlockReward())
}
