package eth
import (
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/accounts/keystore"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/log"
	"math/big"
	"sort"
	"time"
	"github.com/pkg/errors"
	"math/rand"
)
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}
func (eth *Ethereum) vote(state *state.StateDB, addr common.Address, amount *big.Int, strategy *VoterStrategy, producers types.Producers) error {
	account := accounts.Account{Address: addr}
	err := fetchKeystore(eth.AccountManager()).Unlock(account, strategy.Password)
	wallet, err := eth.AccountManager().Find(account)
	if err != nil {
		log.Error("Can not find account:", "addr", addr.Hex())
		return err
	}
	nonce := state.GetNonce(addr)
	tx := types.NewVoteCreationEx(producers, nonce, strategy.GasPrice, amount)
	var chainID *big.Int
	if config := eth.BlockChain().Config(); config.IsEIP155(eth.BlockChain().CurrentHeader().Number) {
		chainID = config.ChainId
	}
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		log.Error("Sign tx failed", "err", err)
		return err
	}
	if err := eth.TxPool().AddLocal(signed); err != nil {
		if err == core.ErrReplaceUnderpriced {
			log.Info("Already in pool", "err", err)
		} else {
			log.Error("Add pool", "err", err)
		}
		return err
	}
	log.Info("Autovote", "hash", signed.Hash().Hex())
	return nil
}
func (eth *Ethereum) doVoteStrategy() (err error) {
	eth.voteLock.RLock()
	defer eth.voteLock.RUnlock()
	if eth.voteStrategy == nil || len(eth.voteStrategy) <= 0 {
		return errors.New("empty vote strategy")
	}
	header := eth.BlockChain().CurrentHeader()
	statedb, err := eth.BlockChain().StateAt(header.Root)
	if err != nil {
		log.Error("Load state failed.")
		return errors.New("load state failed")
	}
	var begin_block_number uint64 = 0
	if header.Number.Uint64() >= common.LEADER_NUMBER {
		begin_block_number = header.Number.Uint64() - common.LEADER_NUMBER
	}
	addrMap := map[common.Address]*big.Int{}
	for i := 0; header != nil && header.Number.Uint64() > begin_block_number; header = eth.BlockChain().GetHeader(header.ParentHash, header.Number.Uint64()-1) {
		block := eth.blockchain.GetBlockByHash(header.Hash())
		if block == nil {
			log.Error("Can not load block", "number", i)
			continue
		}
		signer := types.MakeSigner(eth.chainConfig, block.Header().Number)
		balance := big.NewInt(0).Set(statedb.GetBalance(block.Coinbase()))
		balance.Add(balance, statedb.GetFreeze(block.Coinbase()))
		addrMap[block.Coinbase()] = balance
		for _, tx := range block.Transactions() {
			from, _ := types.Sender(signer, tx)
			balance := big.NewInt(0).Set(statedb.GetBalance(from))
			balance.Add(balance, statedb.GetFreeze(from))
			addrMap[from] = balance
		}
		i++
	}
	producers := types.Producers{}
	for addr, vote := range addrMap {
		producers = append(producers, types.Producer{Addr: addr, Vote: vote})
	}
	sort.Sort(producers) 
	if len(producers) > common.LEADER_LIMIT {
		producers = producers[:common.LEADER_LIMIT]
	}
	if len(producers) <= 0 {
		return errors.New("no producer.")
	}
	for addr, strategy := range eth.voteStrategy {
		b := big.NewInt(0).Set(statedb.GetBalance(addr))
		if b.Cmp(common.VOTE_MONEY_LIMIT) <= 0 { 
			continue
		}
		tx := types.NewVoteCreationEx(producers, 0, strategy.GasPrice, b)
		fee := new(big.Int).Set(tx.GasPrice())
		fee.Mul(fee, big.NewInt(int64(tx.Gas())))
		fee.Mul(fee, big.NewInt(common.LEADER_LIMIT))
		b.Sub(b, fee)
		if strategy.Producer == nil {
			b.Div(b, big.NewInt(int64(len(producers))))
			eth.vote(statedb, addr, b, &strategy, producers)
		} else {
			eth.vote(statedb, addr, b, &strategy, types.Producers{types.Producer{Addr: *strategy.Producer}})
		}
	}
	return nil
}
func (eth *Ethereum) autoVoteLoop() {
	duration, _ := time.ParseDuration("3s")
	rs := rand.NewSource(time.Now().Unix())
	r := rand.New(rs)
	targetNumber := r.Intn(200)
	var vote_round uint64 = 0
	for {
		select {
		case <-eth.shutdownChan:
			return
		default:
			sync := eth.Downloader().Progress()
			syncing := eth.BlockChain().CurrentHeader().Number.Uint64() < sync.HighestBlock
			if !syncing {
				round := common.GetRoundNumberByBlockNumber(eth.BlockChain().CurrentHeader().Number.Uint64())
				number := eth.BlockChain().CurrentHeader().Number.Uint64() % common.LEADER_NUMBER
				if round > vote_round && number >= uint64(targetNumber) {
					err := eth.doVoteStrategy()
					if err == nil {
						vote_round = round
					}
				}
			}
			time.Sleep(duration)
		}
	}
}
func (eth *Ethereum) AddVoter(address common.Address, password string, producer *common.Address, gasPrice *big.Int) {
	eth.voteLock.Lock()
	defer eth.voteLock.Unlock()
	if eth.voteStrategy == nil {
		eth.voteStrategy = map[common.Address]VoterStrategy{}
	}
	eth.voteStrategy[address] = VoterStrategy{Password: password, GasPrice: gasPrice, Producer: producer}
}
func (eth *Ethereum) CleanVoter() {
	eth.voteLock.Lock()
	defer eth.voteLock.Unlock()
	eth.voteStrategy = map[common.Address]VoterStrategy{}
	log.Info("AutoVote stop.")
}
func (eth *Ethereum) IsVoting() bool {
	eth.voteLock.Lock()
	defer eth.voteLock.Unlock()
	return len(eth.voteStrategy) > 0
}
func (eth *Ethereum) startAutoVote() {
	go eth.autoVoteLoop()
}
func (eth *Ethereum) closeAutoVote() {
	log.Info("Auto autovote close")
}
