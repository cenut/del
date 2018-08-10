package eth
import (
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/common"
	"math/big"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core"
	"time"
	"github.com/DEL-ORG/del/params"
	"github.com/pkg/errors"
)
func (eth *Ethereum)active(coinbase common.Address, password string, gasPrice *big.Int) (err error) {
	eth.activeLock.RLock()
	defer eth.activeLock.RUnlock()
	if !eth.activeStart {
		return errors.New("Active not start.")
	}
	account := accounts.Account{Address: coinbase}
	err = fetchKeystore(eth.AccountManager()).Unlock(account, password)
	if err != nil {
		log.Error("Fetch keystore", "err", err)
		return
	}
	wallet, err := eth.AccountManager().Find(account)
	if err != nil {
		log.Error("Can not find account:", "addr", coinbase.Hex(), "err", err)
		return err
	}
	header := eth.BlockChain().CurrentHeader()
	statedb, err := eth.BlockChain().StateAt(header.Root)
	if err != nil {
		log.Error("Load state failed.", "err", err)
		return err
	}
	nonce := statedb.GetNonce(coinbase)
	tx := types.NewTransaction(nonce, coinbase, big.NewInt(0), params.TxGas, gasPrice ,[]byte{})
	var chainID *big.Int
	if config := eth.BlockChain().Config(); config.IsEIP155(eth.BlockChain().CurrentHeader().Number) {
		chainID = config.ChainId
	}
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		log.Error("Sign tx failed", "err", err)
		return err
	}
	if eth.TxPool().Get(tx.Hash()) == nil {
		if err := eth.TxPool().AddLocal(signed); err != nil {
			if err == core.ErrReplaceUnderpriced {
				log.Info("Already in pool", "err", err)
				return nil
			}else {
				log.Error("Add pool", "err", err)
			}
			return err
		}
	}
	log.Info("Active", "coinbase", coinbase, "hash", tx.Hash().Hex())
	return nil
}
func (eth *Ethereum)activeLoop() {
	duration, _ := time.ParseDuration("2s")
	var active_round uint64 = 0
	for {
		select {
		case <- eth.shutdownChan:
			return
		default:
			round := common.GetRoundNumberByBlockNumber(eth.BlockChain().CurrentHeader().Number.Uint64())
			if round > active_round {
				err := eth.active(eth.activeAddr, eth.activePassword, eth.activeGasPrice)
				if err == nil {
					active_round = round
				}
			}
			time.Sleep(duration)
		}
	}
}
func (eth *Ethereum)StartAutoActive(address common.Address, password string, gasPrice *big.Int) {
	eth.activeLock.Lock()
	defer eth.activeLock.Unlock()
	eth.activeAddr = address
	eth.activePassword = password
	eth.activeGasPrice = gasPrice
	eth.activeStart = true
}
func (eth *Ethereum)StopAutoActive() {
	eth.activeLock.Lock()
	defer eth.activeLock.Unlock()
	eth.activeStart = false
	log.Info("AutoActive stop.")
}
func (eth *Ethereum)IsActiving() bool {
	eth.activeLock.RLock()
	defer eth.activeLock.RUnlock()
	return eth.activeStart
}
func (eth *Ethereum) startActive() {
	go eth.activeLoop()
}
func (eth *Ethereum)closeActive() {
	log.Info("Auto active close")
}
