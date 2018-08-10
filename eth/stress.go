package eth
import (
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/log"
	"math/big"
	"sync/atomic"
	"time"
)
var COINBASE = common.HexToAddress("0x49750ff268e80f21c954e143c35f7251d7f4bb56")
const (
	PRIVATE      = "0ce3aec3dfc27a3dfa96ac55d1eb2b6ba221dfa0b454a6b2d12fddd3b2d60741"
	PASSWORD     = "123456"
	ROBOT_NUMBER = 100
)


func (s *Ethereum) doRobot(wallets []accounts.Wallet, coinbase_wallet accounts.Wallet, coinbase_account accounts.Account, gasLimit uint64, gasPrice *big.Int) {
	s.wgStopStressTesting.Add(1)
	defer s.wgStopStressTesting.Add(-1)
	s.TxPool().State()
	log.Info("StressTesting start")
	for {
		for index, wallet := range wallets {
			select {
			case <-s.stopStressTesting:
				return
			default:
			}
			if index%10 == 0 {
				for {
					pending, err := s.txPool.Pending()
					if err != nil || pending == nil {
						break
					}
					total := 0
					for _, pending_txs := range pending {
						total += len(pending_txs)
					}
					if total < 1000 {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			account := wallet.Accounts()[0]
			statedb := s.TxPool().State()
			nonce := statedb.GetNonce(coinbase_account.Address)
			tx := types.NewTransaction(nonce, account.Address, big.NewInt(1), gasLimit, gasPrice, []byte{})
			chainID := s.BlockChain().Config().ChainId
			tx.Hash()
			if s.TxPool().Get(tx.Hash()) != nil {
				continue
			}
			signed, err := coinbase_wallet.SignTx(coinbase_account, tx, chainID)
			if err != nil {
				log.Crit("SignTx", "err", err)
				break
			}
			if err := s.TxPool().AddLocal(signed); err != nil {
				if err == core.ErrReplaceUnderpriced {
					log.Info("Already in pool", "err", err, "hash", tx.Hash())
				} else {
					log.Error("Add pool", "err", err)
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond)
	}
	log.Info("StressTesting stop")
}
func (s *Ethereum) StopStressTesting() {
	if atomic.LoadInt32(&s.stressTestingCount) <= 0 {
		log.Info("StressTesting already stopped")
		return
	}
	close(s.stopStressTesting)
	s.wgStopStressTesting.Wait()
	atomic.AddInt32(&s.stressTestingCount, -1)
	return
}
func (s *Ethereum) StartStressTesting(gasLimit uint64, gasPrice *big.Int) {
	if atomic.LoadInt32(&s.stressTestingCount) > 0 {
		return
	}
	s.stopStressTesting = make(chan bool)
	atomic.AddInt32(&s.stressTestingCount, 1)
	keyStore := fetchKeystore(s.AccountManager())
	wallets := keyStore.Wallets()
	coinbase_account := accounts.Account{}
	var coinbase_wallet accounts.Wallet
	for _, wallet := range wallets {
		account := wallet.Accounts()[0]
		if account.Address == COINBASE {
			coinbase_account = account
			coinbase_wallet = wallet
		}
	}
	tstart := time.Now()
	wallet_len := 0
	for _, wallet := range wallets {
		wallet_len += len(wallet.Accounts())
	}
	users := []accounts.Account{}
	for i := wallet_len; i < ROBOT_NUMBER; i++ {
		key, err := keyStore.NewAccount(PASSWORD)
		if err != nil {
			log.Crit("NewAccount", "err", err)
		}
		users = append(users, key)
	}
	taccount := time.Now()
	duration := taccount.Unix() - tstart.Unix()
	log.Info("NewAccount", "count", ROBOT_NUMBER-wallet_len, "elapsed", duration)
	err := keyStore.Unlock(accounts.Account{Address: COINBASE}, PASSWORD)
	if err != nil {
		log.Crit("Unlock", "err", err)
	}
	for _, wallet := range wallets {
		account := wallet.Accounts()[0]
		err := keyStore.Unlock(accounts.Account{Address: account.Address}, PASSWORD)
		if err != nil {
			log.Crit("Unlock", "err", err)
		}
	}
	tunlock := time.Now()
	duration = tunlock.Unix() - tstart.Unix()
	log.Info("UnlockAccount", "elapsed", duration)
	go s.doRobot(wallets, coinbase_wallet, coinbase_account, gasLimit, gasPrice)
	return
}
