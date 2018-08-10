package ethapi
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"encoding/hex"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/accounts/keystore"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/common/math"
	"github.com/DEL-ORG/del/consensus/ethash"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core/vm"
	"github.com/DEL-ORG/del/crypto"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/p2p"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/rlp"
	"github.com/DEL-ORG/del/rpc"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)
const (
	defaultGasPrice = 50 * params.Shannon
)
type PublicEthereumAPI struct {
	b Backend
}
func NewPublicEthereumAPI(b Backend) *PublicEthereumAPI {
	return &PublicEthereumAPI{b}
}
func (s *PublicEthereumAPI) GasPrice(ctx context.Context) (*big.Int, error) {
	return s.b.SuggestPrice(ctx)
}
func (s *PublicEthereumAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(s.b.ProtocolVersion())
}
func (s *PublicEthereumAPI) Syncing() (interface{}, error) {
	progress := s.b.Downloader().Progress()
	if progress.CurrentBlock >= progress.HighestBlock {
		return false, nil
	}
	return map[string]interface{}{
		"startingBlock": hexutil.Uint64(progress.StartingBlock),
		"currentBlock":  hexutil.Uint64(progress.CurrentBlock),
		"highestBlock":  hexutil.Uint64(progress.HighestBlock),
		"pulledStates":  hexutil.Uint64(progress.PulledStates),
		"knownStates":   hexutil.Uint64(progress.KnownStates),
		"duration":      hexutil.Uint64((progress.HighestBlock - progress.CurrentBlock) * common.SLOT_BASE),
	}, nil
}
type PublicTxPoolAPI struct {
	b Backend
}
func NewPublicTxPoolAPI(b Backend) *PublicTxPoolAPI {
	return &PublicTxPoolAPI{b}
}
func (s *PublicTxPoolAPI) Content() map[string]map[string]map[string]*RPCTransaction {
	content := map[string]map[string]map[string]*RPCTransaction{
		"pending": make(map[string]map[string]*RPCTransaction),
		"queued":  make(map[string]map[string]*RPCTransaction),
	}
	pending, queue := s.b.TxPoolContent()
	for account, txs := range pending {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	for account, txs := range queue {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}
func (s *PublicTxPoolAPI) Status() map[string]hexutil.Uint {
	pending, queue := s.b.Stats()
	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(pending),
		"queued":  hexutil.Uint(queue),
	}
}
func (s *PublicTxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}
	pending, queue := s.b.TxPoolContent()
	var format = func(tx *types.Transaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v wei + %v gas × %v wei", tx.To().Hex(), tx.Value(), tx.Gas(), tx.GasPrice())
		}
		return fmt.Sprintf("contract creation: %v wei + %v gas × %v wei", tx.Value(), tx.Gas(), tx.GasPrice())
	}
	for account, txs := range pending {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	for account, txs := range queue {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}
type PublicAccountAPI struct {
	am *accounts.Manager
}
func NewPublicAccountAPI(am *accounts.Manager) *PublicAccountAPI {
	return &PublicAccountAPI{am: am}
}
func (s *PublicAccountAPI) Accounts() []common.Address {
	addresses := make([]common.Address, 0) 
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address)
		}
	}
	return addresses
}
type PrivateAccountAPI struct {
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         Backend
}
func NewPrivateAccountAPI(b Backend, nonceLock *AddrLocker) *PrivateAccountAPI {
	return &PrivateAccountAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
	}
}
func (s *PrivateAccountAPI) ListAccounts() []common.Address {
	addresses := make([]common.Address, 0) 
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address)
		}
	}
	return addresses
}
type rawWallet struct {
	URL      string             `json:"url"`
	Status   string             `json:"status"`
	Failure  string             `json:"failure,omitempty"`
	Accounts []accounts.Account `json:"accounts,omitempty"`
}
func (s *PrivateAccountAPI) ListWallets() []rawWallet {
	wallets := make([]rawWallet, 0) 
	for _, wallet := range s.am.Wallets() {
		status, failure := wallet.Status()
		raw := rawWallet{
			URL:      wallet.URL().String(),
			Status:   status,
			Accounts: wallet.Accounts(),
		}
		if failure != nil {
			raw.Failure = failure.Error()
		}
		wallets = append(wallets, raw)
	}
	return wallets
}
func (s *PrivateAccountAPI) OpenWallet(url string, passphrase *string) error {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return err
	}
	pass := ""
	if passphrase != nil {
		pass = *passphrase
	}
	return wallet.Open(pass)
}
func (s *PrivateAccountAPI) DeriveAccount(url string, path string, pin *bool) (accounts.Account, error) {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return accounts.Account{}, err
	}
	derivPath, err := accounts.ParseDerivationPath(path)
	if err != nil {
		return accounts.Account{}, err
	}
	if pin == nil {
		pin = new(bool)
	}
	return wallet.Derive(derivPath, *pin)
}
func (s *PrivateAccountAPI) ExportRawKey(addr common.Address) (privateKey string, err error) {
	key, err := fetchKeystore(s.am).GetPrivateKey(addr)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(crypto.FromECDSA(key.PrivateKey)), nil
}
func (s *PrivateAccountAPI) NewAccount(password string) (common.Address, error) {
	acc, err := fetchKeystore(s.am).NewAccount(password)
	if err == nil {
		return acc.Address, nil
	}
	return common.Address{}, err
}
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}
func (s *PrivateAccountAPI) ImportRawKey(privkey string, password string) (common.Address, error) {
	key, err := crypto.HexToECDSA(privkey)
	if err != nil {
		return common.Address{}, err
	}
	acc, err := fetchKeystore(s.am).ImportECDSA(key, password)
	return acc.Address, err
}
func (s *PrivateAccountAPI) UnlockAccount(addr common.Address, password string, duration *uint64) (bool, error) {
	const max = uint64(time.Duration(math.MaxInt64) / time.Second)
	var d time.Duration
	if duration == nil {
		d = 3600 * time.Second
	} else if *duration > max {
		return false, errors.New("unlock duration too large")
	} else {
		d = time.Duration(*duration) * time.Second
	}
	err := fetchKeystore(s.am).TimedUnlock(accounts.Account{Address: addr}, password, d)
	return err == nil, err
}
func (s *PrivateAccountAPI) LockAccount(addr common.Address) bool {
	return fetchKeystore(s.am).Lock(addr) == nil
}
func (s *PrivateAccountAPI) signTransaction(ctx context.Context, args SendTxArgs, passwd string) (*types.Transaction, error) {
	account := accounts.Account{Address: args.From}
	wallet, err := s.am.Find(account)
	if err != nil {
		return nil, err
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	tx := args.toTransaction()
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	return wallet.SignTxWithPassphrase(account, passwd, tx, chainID)
}
func (s *PrivateAccountAPI) SendTransaction(ctx context.Context, args SendTxArgs, passwd string) (common.Hash, error) {
	if args.Nonce == nil {
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, signed)
}
func (s *PrivateAccountAPI) SignTransaction(ctx context.Context, args SendTxArgs, passwd string) (*SignTransactionResult, error) {
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return nil, err
	}
	data, err := rlp.EncodeToBytes(signed)
	if err != nil {
		return nil, err
	}
	return &SignTransactionResult{data, signed}, nil
}
func signHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}
func (s *PrivateAccountAPI) Sign(ctx context.Context, data hexutil.Bytes, addr common.Address, passwd string) (hexutil.Bytes, error) {
	account := accounts.Account{Address: addr}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	signature, err := wallet.SignHashWithPassphrase(account, passwd, signHash(data))
	if err != nil {
		return nil, err
	}
	signature[64] += 27 
	return signature, nil
}
func (s *PrivateAccountAPI) EcRecover(ctx context.Context, data, sig hexutil.Bytes) (common.Address, error) {
	if len(sig) != 65 {
		return common.Address{}, fmt.Errorf("signature must be 65 bytes long")
	}
	if sig[64] != 27 && sig[64] != 28 {
		return common.Address{}, fmt.Errorf("invalid Ethereum signature (V is not 27 or 28)")
	}
	sig[64] -= 27 
	rpk, err := crypto.Ecrecover(signHash(data), sig)
	if err != nil {
		return common.Address{}, err
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return recoveredAddr, nil
}
func (s *PrivateAccountAPI) SignAndSendTransaction(ctx context.Context, args SendTxArgs, passwd string) (common.Hash, error) {
	return s.SendTransaction(ctx, args, passwd)
}
type PublicBlockChainAPI struct {
	b Backend
}
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}
func (s *PublicBlockChainAPI) BlockNumber() *big.Int {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) 
	return header.Number
}
func (s *PublicBlockChainAPI) GetBeginBlockNumberByRoundNumber(ctx context.Context, roundNumber uint64) uint64 {
	return common.GetBeginBlockNumberByRoundNumber(roundNumber)
}
func (s *PublicBlockChainAPI) GetEndBlockNumberByRoundNumber(ctx context.Context, roundNumber uint64) uint64 {
	return common.GetEndBlockNumberByRoundNumber(roundNumber)
}
func (s *PublicBlockChainAPI) GetRoundNumberByBlockNumber(ctx context.Context, blockNr rpc.BlockNumber) uint64 {
	var number uint64 = 0
	if blockNr == rpc.LatestBlockNumber || blockNr < 0 || int64(blockNr) > s.b.CurrentBlock().Number().Int64() {
		number = s.b.CurrentBlock().NumberU64()
	} else {
		number = uint64(blockNr)
	}
	return common.GetRoundNumberByBlockNumber(number)
}
type ADataProtocolVote struct {
	Addr   string       `json:"addr" gencodec:"required"`
	Amount *hexutil.Big `json:"amount,omitempty" gencodec:"required"`
}
type ADataProtocolTickets []ADataProtocolVote
type ADataProtocol struct {
	MessageID uint16  `json:"message_id" gencodec:"required"`
	Text      *string `json:"text,omitempty" gencodec:"required"`
	Tickets ADataProtocolTickets `json:"tickets,omitempty" gencodec:"required"`
}
func (s *PublicBlockChainAPI) GetDayRewardEx(ctx context.Context, addr common.Address) (reward *types.OutputBlockReward, err error) {
	return s.b.Get24HRewardEx(addr)
}
func (s *PublicBlockChainAPI) GetDayReward(ctx context.Context, addr common.Address) (reward *big.Int, err error) {
	return s.b.Get24HReward(addr)
}
func (s *PublicBlockChainAPI) DecodeMessage(ctx context.Context, buff []byte) (message *ADataProtocol, err error) {
	m, err := common.NewDataProtocol(buff)
	if err != nil {
		return nil, err
	}
	message = &ADataProtocol{}
	message.MessageID = m.MessageID
	message.Text = m.Text
	for _, ticket := range m.Tickets {
		message.Tickets = append(message.Tickets, ADataProtocolVote{
			Addr:   ticket.Addr,
			Amount: (*hexutil.Big)(ticket.Amount),
		})
	}
	return message, nil
}
func (s *PublicBlockChainAPI) MakeVoteMessage(ctx context.Context, tickets ADataProtocolTickets) ([]byte, error) {
	msg := common.DataProtocol{}
	msg.MessageID = common.DataProtocolMessageID_VOTE
	msg.Tickets = nil
	for _, ticket := range tickets {
		msg.Tickets = append(msg.Tickets, common.DataProtocolVote{Addr: ticket.Addr, Amount: (*big.Int)(ticket.Amount)})
	}
	return msg.Encode()
}
func (s *PublicBlockChainAPI) MakeTextMessage(ctx context.Context, text string) ([]byte, error) {
	msg := common.DataProtocol{}
	msg.MessageID = common.DataProtocolMessageID_TEXT
	msg.Text = &text
	return msg.Encode()
}
func (s *PublicBlockChainAPI) GetPoolNonce(ctx context.Context, address common.Address) (uint64, error) {
	return s.b.GetPoolNonce(ctx, address)
}
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	b := state.GetBalance(address)
	return b, state.Error()
}
func (s *PublicBlockChainAPI) GetVoterFreeze(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (freeze *big.Int, err error) {
	return s.b.GetVoteFreeze(ctx, address, blockNr)
}
func (s *PublicBlockChainAPI) GetVoterState(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (voter *types.OVoter, err error) {
	header, err := s.b.HeaderByNumber(ctx, blockNr)
	if err != nil || header == nil {
		return nil, err
	}
	votersMap, err := s.b.GetVotersState(ctx, header)
	if err != nil {
		return nil, nil
	}
	if votersMap == nil {
		return nil, nil
	}
	producers := votersMap.GetProducers()
	if producers == nil {
		return nil, nil
	}
	for i := 0; i < len(producers); i++ {
		producer := producers[i]
		if producer.Addr == address {
			return &types.OVoter{Vote: producer.Vote, Rank: uint64(i + 1)}, nil
		}
	}
	return nil, nil
}
func (s *PublicBlockChainAPI) GetVoterReward(ctx context.Context, count int64, address []common.Address) (rewards []types.OutputReward, err error) {
	total := 0
	for number := s.b.CurrentBlock().Number().Uint64(); number > 0; number-- {
		total++
		if total > common.LEADER_LIMIT*2 {
			return
		}
		header, err := s.b.HeaderByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			continue
		}
		total := big.NewInt(0)
		for _, addr := range address {
			r := s.b.GetVoterReward(number, addr)
			if r == nil {
				log.Error("VoterReward not found", "number", number, "addr", addr)
				continue
			}
			total.Add(total, r)
		}
		if total.Cmp(common.Big0) > 0 {
			rewards = append(rewards, types.OutputReward{
				Time:   time.Unix(header.Time.Int64(), 0),
				Number: number,
				Amount: total,
			})
			if count > 0 && int64(len(rewards)) >= count {
				break
			}
		}
	}
	return rewards, nil
}
func (s *PublicBlockChainAPI) GetBlockRewardByNumber(ctx context.Context, address common.Address, number uint64) (reward types.OutputBlockReward, err error) {
	round := common.GetRoundNumberByBlockNumber(number)
	beginBlockNumber := common.GetBeginBlockNumberByRoundNumber(round)
	reward.CoinbaseReward = big.NewInt(0)
	reward.SuperCoinbaseReward = big.NewInt(0)
	reward.BlockReward = big.NewInt(0)
	reward.VoteReward = big.NewInt(0)
	reward.Number = beginBlockNumber
	for ; number >= beginBlockNumber; number-- {
		header, err := s.b.HeaderByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			continue
		}
		rewardConfig := ethash.GetRewardByNumber(number)
		blockReward := common.Big0
		if header.Coinbase == address {
			blockReward = rewardConfig.GetBlockReward()
		}
		superCoinbaseReward := s.b.GetSuperCoinbaseReward(number, address)
		if superCoinbaseReward == nil {
			log.Error("superCoinbaseReward not found", "number", number, "addr", address)
			superCoinbaseReward = common.Big0
		}
		coinbaseReward := s.b.GetCoinbaseReward(number, address)
		if coinbaseReward == nil {
			log.Error("CoinbaseReward not found", "number", number, "addr", address)
			coinbaseReward = common.Big0
		}
		voteReward := s.b.GetVoterReward(number, address)
		if voteReward == nil {
			log.Error("CoinbaseReward not found", "number", number, "addr", address)
			voteReward = common.Big0
		}
		reward.SuperCoinbaseReward.Add(reward.SuperCoinbaseReward, superCoinbaseReward)
		reward.CoinbaseReward.Add(reward.CoinbaseReward, coinbaseReward)
		reward.VoteReward.Add(reward.VoteReward, voteReward)
		reward.BlockReward.Add(reward.BlockReward, blockReward)
	}
	return reward, nil
}
func (s *PublicBlockChainAPI) GetBlockReward(ctx context.Context, count int64, address common.Address) (rewards []types.OutputBlockReward, err error) {
	total := 0
	for number := s.b.CurrentBlock().Number().Uint64(); number > 0; number-- {
		total++
		if total > common.LEADER_LIMIT*2 {
			return
		}
		header, err := s.b.HeaderByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			continue
		}
		rewardConfig := ethash.GetRewardByNumber(number)
		blockReward := common.Big0
		if header.Coinbase == address {
			blockReward = rewardConfig.GetBlockReward()
		}
		superCoinbaseReward := s.b.GetSuperCoinbaseReward(number, address)
		if superCoinbaseReward == nil {
			log.Error("superCoinbaseReward not found", "number", number, "addr", address)
			superCoinbaseReward = common.Big0
		}
		coinbaseReward := s.b.GetCoinbaseReward(number, address)
		if coinbaseReward == nil {
			log.Error("CoinbaseReward not found", "number", number, "addr", address)
			coinbaseReward = common.Big0
		}
		voteReward := s.b.GetVoterReward(number, address)
		if voteReward == nil {
			log.Error("CoinbaseReward not found", "number", number, "addr", address)
			voteReward = common.Big0
		}
		reward := types.OutputBlockReward{
			Time:                time.Unix(header.Time.Int64(), 0),
			Number:              number,
			SuperCoinbaseReward: superCoinbaseReward,
			CoinbaseReward:      coinbaseReward,
			VoteReward:          voteReward,
			BlockReward:         blockReward,
		}
		if reward.SuperCoinbaseReward.Cmp(common.Big0) > 0 || reward.CoinbaseReward.Cmp(common.Big0) > 0 ||
			reward.VoteReward.Cmp(common.Big0) > 0 || reward.BlockReward.Cmp(common.Big0) > 0 {
			rewards = append(rewards, reward)
			if count > 0 && int64(len(rewards)) >= count {
				break
			}
		}
	}
	return rewards, nil
}
func (s *PublicBlockChainAPI) GetSuperCoinbaseReward(ctx context.Context, count int64, address []common.Address) (rewards []types.OutputReward, err error) {
	total := 0
	for number := s.b.CurrentBlock().Number().Uint64(); number > 0; number-- {
		total++
		if total > common.LEADER_LIMIT*2 {
			return
		}
		header, err := s.b.HeaderByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			continue
		}
		total := big.NewInt(0)
		for _, addr := range address {
			r := s.b.GetSuperCoinbaseReward(number, addr)
			if r == nil {
				log.Error("CoinbaseReward not found", "number", number, "addr", addr)
				continue
			}
			total.Add(total, r)
		}
		if total.Cmp(common.Big0) > 0 {
			rewards = append(rewards, types.OutputReward{
				Time:   time.Unix(header.Time.Int64(), 0),
				Number: number,
				Amount: total,
			})
			if count > 0 && int64(len(rewards)) >= count {
				break
			}
		}
	}
	return rewards, nil
}
func (s *PublicBlockChainAPI) GetCoinbaseReward(ctx context.Context, count int64, address []common.Address) (rewards []types.OutputReward, err error) {
	total := 0
	for number := s.b.CurrentBlock().Number().Uint64(); number > 0; number-- {
		total++
		if total > common.LEADER_LIMIT*2 {
			return
		}
		header, err := s.b.HeaderByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			continue
		}
		total := big.NewInt(0)
		for _, addr := range address {
			r := s.b.GetCoinbaseReward(number, addr)
			if r == nil {
				log.Error("CoinbaseReward not found", "number", number, "addr", addr)
				continue
			}
			total.Add(total, r)
		}
		if total.Cmp(common.Big0) > 0 {
			rewards = append(rewards, types.OutputReward{
				Time:   time.Unix(header.Time.Int64(), 0),
				Number: number,
				Amount: total,
			})
			if count > 0 && int64(len(rewards)) >= count {
				break
			}
		}
	}
	return rewards, nil
}
func (s *PublicBlockChainAPI) GetLastTexts(ctx context.Context, count int64, address []common.Address) (texts []types.OutputText, err error) {
	if address == nil || len(address) <= 0 {
		address = append(address, common.OFFICIAL)
	}
	texts = nil
	total := 0
search:
	for number := s.b.CurrentBlock().Number().Int64(); number > 0; number = number - 1 {
		total++
		if total > common.LEADER_LIMIT*2 {
			break
		}
		block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			return nil, err
		}
		signer := types.MakeSigner(s.b.ChainConfig(), block.Number())
		for i := block.Transactions().Len() - 1; i >= 0; i = i - 1 {
			tx := block.Transactions()[i]
			message, merr := tx.GetMessage()
			if merr != nil {
				continue
			}
			from, _ := types.Sender(signer, tx)
			okey := false
			for _, addr := range address {
				if addr == from {
					okey = true
				}
				if tx.To() != nil && addr == *tx.To() {
					okey = true
				}
			}
			if !okey {
				continue
			}
			if message.MessageID == common.DataProtocolMessageID_TEXT && message.Text != nil {
				price := new(big.Int).Set(tx.GasPrice())
				price.Mul(price, new(big.Int).SetUint64(tx.Gas()))
				texts = append(texts, types.OutputText{
					From:  from,
					To:    *tx.To(),
					Price: price,
					Time:  time.Unix(block.Time().Int64(), 0),
					Hash:  tx.Hash(),
					Text:  *message.Text,
				})
				if count > 0 && int64(len(texts)) >= count {
					break search
				}
			}
		}
	}
	return texts, nil
}
func (s *PublicBlockChainAPI) GetLastTxs(ctx context.Context, count int64, address []common.Address) (otxs []types.OutputTx, err error) {
	otxs = nil
	total := 0
	if address == nil || len(address) <= 0 {
		for _, wallet := range s.b.AccountManager().Wallets() {
			for _, account := range wallet.Accounts() {
				address = append(address, account.Address)
			}
		}
	}
search:
	for number := s.b.CurrentBlock().Number().Int64(); number > 0; number = number - 1 {
		total++
		if total > common.LEADER_LIMIT*2 {
			break
		}
		block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(number))
		if err != nil {
			return nil, err
		}
		signer := types.MakeSigner(s.b.ChainConfig(), block.Number())
		for i := block.Transactions().Len() - 1; i >= 0; i = i - 1 {
			tx := block.Transactions()[i]
			if tx.To() == nil {
				continue
			}
			from, _ := types.Sender(signer, tx)
			in := false
			out := false
			for _, addr := range address {
				if addr == from {
					out = true
				}
				if tx.To() != nil && addr == *tx.To() {
					in = true
				}
			}
			if !in && !out {
				continue
			}
			price := new(big.Int).Set(tx.GasPrice())
			price.Mul(price, new(big.Int).SetUint64(tx.Gas()))
			otx := &types.OutputTx{
				Hash:  tx.Hash(),
				From:  from,
				To:    *tx.To(),
				Time:  time.Unix(block.Header().Time.Int64(), 0),
				Type:  common.TXTYPE_TRANSFER,
				Price: price,
				Value: tx.Value(),
			}
			if out {
				otx.Value.Mul(otx.Value, big.NewInt(-1))
			}
			if tx.Value().Cmp(common.Big0) == 0 {
				message, error := tx.GetMessage()
				if error != nil {
					continue
				}
				if message.MessageID == common.DataProtocolMessageID_TEXT {
					otx.Type = common.TXTYPE_TEXT
				} else if message.MessageID == common.DataProtocolMessageID_VOTE {
					otx.Type = common.TXTYPE_VOTE
				} else {
					otx = nil
				}
			}
			if otx != nil {
				otxs = append(otxs, *otx)
				if count > 0 && int64(len(otxs)) >= count {
					break search
				}
			}
		}
	}
	return otxs, nil
}
func (s *PublicBlockChainAPI) CheckSuperProducer(ctx context.Context, address common.Address) bool {
	producers, err := s.b.GetProducers(ctx, rpc.LatestBlockNumber, true)
	if producers == nil || err != nil {
		return false
	}
	for i := 0; i < len(producers) && i < common.SUPER_COINBASE_RANK; i++ {
		producer := producers[i]
		if producer.Addr == address {
			return true
		}
	}
	return false
}
func (s *PublicBlockChainAPI) CheckProducer(ctx context.Context, address common.Address) bool {
	producers, err := s.b.GetProducers(ctx, rpc.LatestBlockNumber, true)
	if producers == nil || err != nil {
		return false
	}
	for _, producer := range producers {
		if producer.Addr == address {
			return true
		}
	}
	return false
}
func (s *PublicBlockChainAPI) GetVoters(ctx context.Context, blockNr rpc.BlockNumber) (types.Voters, error) {
	return s.b.GetVoters(ctx, blockNr)
	
}
func (s *PublicBlockChainAPI) GetProducers(ctx context.Context, blockNr rpc.BlockNumber, hidden bool) ([]types.Producer, error) {
	return s.b.GetProducers(ctx, blockNr, hidden)
	
}
func (s *PublicBlockChainAPI) GetFreeze(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, errors.New("Block not exists.")
	}
	b := state.GetFreeze(address)
	return b, state.Error()
}
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		response, err := s.rpcOutputBlock(block, true, fullTx)
		if err == nil && blockNr == rpc.PendingBlockNumber {
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		return s.rpcOutputBlock(block, true, fullTx)
	}
	return nil, err
}
func (s *PublicBlockChainAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			log.Debug("Requested uncle not found", "number", blockNr, "hash", block.Hash(), "index", index)
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}
func (s *PublicBlockChainAPI) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			log.Debug("Requested uncle not found", "number", block.Number(), "hash", blockHash, "index", index)
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}
func (s *PublicBlockChainAPI) GetUncleCountByBlockNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		n := hexutil.Uint(len(block.Uncles()))
		return &n
	}
	return nil
}
func (s *PublicBlockChainAPI) GetUncleCountByBlockHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Uncles()))
		return &n
	}
	return nil
}
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, key string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}
type CallArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      hexutil.Uint64  `json:"gas"`
	GasPrice hexutil.Big     `json:"gasPrice"`
	Value    hexutil.Big     `json:"value"`
	Data     hexutil.Bytes   `json:"data"`
}
func (s *PublicBlockChainAPI) doCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber, vmCfg vm.Config) ([]byte, uint64, bool, *big.Int, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())
	state, header, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, 0, false, big.NewInt(0), err
	}
	addr := args.From
	if addr == (common.Address{}) {
		if wallets := s.b.AccountManager().Wallets(); len(wallets) > 0 {
			if accounts := wallets[0].Accounts(); len(accounts) > 0 {
				addr = accounts[0].Address
			}
		}
	}
	gas, gasPrice := uint64(args.Gas), args.GasPrice.ToInt()
	if gas == 0 {
		gas = 50000000
	}
	if gasPrice.Sign() == 0 {
		gasPrice = new(big.Int).SetUint64(defaultGasPrice)
	}
	msg := types.NewMessage(addr, args.To, 0, args.Value.ToInt(), gas, gasPrice, args.Data, false)
	var cancel context.CancelFunc
	if vmCfg.DisableGasMetering {
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer func() { cancel() }()
	evm, vmError, err := s.b.GetEVM(ctx, msg, state, header, vmCfg)
	if err != nil {
		return nil, 0, false, big.NewInt(0), err
	}
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()
	gp := new(core.GasPool).AddGas(math.MaxUint64)
	res, gas, failed, reward, err := core.ApplyMessage(evm, msg, gp)
	if err := vmError(); err != nil {
		return nil, 0, false, reward, err
	}
	return res, gas, failed, reward, err
}
func (s *PublicBlockChainAPI) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, _, _, err := s.doCall(ctx, args, blockNr, vm.Config{DisableGasMetering: true})
	return (hexutil.Bytes)(result), err
}
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args CallArgs) (hexutil.Uint64, error) {
	var (
		lo  uint64 = params.TxGas - 1
		hi  uint64
		cap uint64
	)
	if uint64(args.Gas) >= params.TxGas {
		hi = uint64(args.Gas)
	} else {
		block, err := s.b.BlockByNumber(ctx, rpc.PendingBlockNumber)
		if err != nil {
			return 0, err
		}
		hi = block.GasLimit()
	}
	cap = hi
	executable := func(gas uint64) bool {
		args.Gas = hexutil.Uint64(gas)
		_, _, failed, _, err := s.doCall(ctx, args, rpc.PendingBlockNumber, vm.Config{})
		if err != nil || failed {
			return false
		}
		return true
	}
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	if hi == cap {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance or always failing transaction")
		}
	}
	return hexutil.Uint64(hi), nil
}
type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   error              `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}
func FormatLogs(logs []vm.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.Err,
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = fmt.Sprintf("%x", math.PaddedBigBytes(stackValue, 32))
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}
func (s *PublicBlockChainAPI) rpcOutputBlock(b *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	head := b.Header() 
	fields := map[string]interface{}{
		"number":           (*hexutil.Big)(head.Number),
		"hash":             b.Hash(),
		"parentHash":       head.ParentHash,
		"nonce":            head.Nonce,
		"mixHash":          head.MixDigest,
		"sha3Uncles":       head.UncleHash,
		"logsBloom":        head.Bloom,
		"stateRoot":        head.Root,
		"miner":            head.Coinbase,
		"difficulty":       (*hexutil.Big)(head.Difficulty),
		"totalDifficulty":  (*hexutil.Big)(s.b.GetTd(b.Hash())),
		"extraData":        hexutil.Bytes(head.Extra),
		"size":             hexutil.Uint64(b.Size()),
		"gasLimit":         hexutil.Uint64(head.GasLimit),
		"gasUsed":          hexutil.Uint64(head.GasUsed),
		"timestamp":        (*hexutil.Big)(head.Time),
		"transactionsRoot": head.TxHash,
		"receiptsRoot":     head.ReceiptHash,
		"producerHash":     head.ProducerHash,
		"voterHash":        head.VoterHash,
	}
	if inclTx {
		formatTx := func(tx *types.Transaction) (interface{}, error) {
			return tx.Hash(), nil
		}
		if fullTx {
			formatTx = func(tx *types.Transaction) (interface{}, error) {
				return newRPCTransactionFromBlockHash(b, tx.Hash()), nil
			}
		}
		txs := b.Transactions()
		transactions := make([]interface{}, len(txs))
		var err error
		for i, tx := range b.Transactions() {
			if transactions[i], err = formatTx(tx); err != nil {
				return nil, err
			}
		}
		fields["transactions"] = transactions
	}
	uncles := b.Uncles()
	uncleHashes := make([]common.Hash, len(uncles))
	for i, uncle := range uncles {
		uncleHashes[i] = uncle.Hash()
	}
	fields["uncles"] = uncleHashes
	return fields, nil
}
type RPCTransaction struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex hexutil.Uint    `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
}
func newRPCTransaction(tx *types.Transaction, blockHash common.Hash, blockNumber uint64, index uint64) *RPCTransaction {
	var signer types.Signer = types.FrontierSigner{}
	if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainId())
	}
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()
	result := &RPCTransaction{
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = hexutil.Uint(index)
	}
	return result
}
func newRPCPendingTransaction(tx *types.Transaction) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, 0, 0)
}
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64) *RPCTransaction {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.Hash(), b.NumberU64(), index)
}
func newRPCRawTransactionFromBlockIndex(b *types.Block, index uint64) hexutil.Bytes {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	blob, _ := rlp.EncodeToBytes(txs[index])
	return blob
}
func newRPCTransactionFromBlockHash(b *types.Block, hash common.Hash) *RPCTransaction {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil
}
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
}
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock}
}
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		n := hexutil.Uint(len(block.Transactions()))
		return &n
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Transactions()))
		return &n
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) *RPCTransaction {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) *RPCTransaction {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) hexutil.Bytes {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) hexutil.Bytes {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	nonce := state.GetNonce(address)
	return (*hexutil.Uint64)(&nonce), state.Error()
}
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *RPCTransaction {
	if tx, blockHash, blockNumber, index := core.GetTransaction(s.b.ChainDb(), hash); tx != nil {
		return newRPCTransaction(tx, blockHash, blockNumber, index)
	}
	if tx := s.b.GetPoolTransaction(hash); tx != nil {
		return newRPCPendingTransaction(tx)
	}
	return nil
}
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	var tx *types.Transaction
	if tx, _, _, _ = core.GetTransaction(s.b.ChainDb(), hash); tx == nil {
		if tx = s.b.GetPoolTransaction(hash); tx == nil {
			return nil, nil
		}
	}
	return rlp.EncodeToBytes(tx)
}
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(hash common.Hash) (map[string]interface{}, error) {
	tx, blockHash, blockNumber, index := core.GetTransaction(s.b.ChainDb(), hash)
	if tx == nil {
		return nil, errors.New("unknown transaction")
	}
	receipt, _, _, _ := core.GetReceipt(s.b.ChainDb(), hash) 
	if receipt == nil {
		return nil, errors.New("unknown receipt")
	}
	var signer types.Signer = types.FrontierSigner{}
	if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainId())
	}
	from, _ := types.Sender(signer, tx)
	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       hexutil.Uint64(blockNumber),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint64(index),
		"from":              from,
		"to":                tx.To(),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	} else {
		fields["status"] = hexutil.Uint(receipt.Status)
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}
func (s *PublicTransactionPoolAPI) sign(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	account := accounts.Account{Address: addr}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	return wallet.SignTx(account, tx, chainID)
}
type SendTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`
}
func (args *SendTxArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
		*(*uint64)(args.Gas) = 90000
	}
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`Both "data" and "input" are set and not equal. Please use "input" to pass transaction call data.`)
	}
	return nil
}
func (args *SendTxArgs) toTransaction() *types.Transaction {
	var input []byte
	if args.Data != nil {
		input = *args.Data
	} else if args.Input != nil {
		input = *args.Input
	}
	if args.To == nil {
		return types.NewContractCreation(uint64(*args.Nonce), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input)
	}
	return types.NewTransaction(uint64(*args.Nonce), *args.To, (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input)
}
type StartAutoActiveArgs struct {
	Addr     common.Address `json:"addr"`
	Password string         `json:"password"`
	GasPrice *hexutil.Big `json:"gasPrice"`
}
func (args *StartAutoActiveArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	return nil
}
type StartAutoVoteArgs struct {
	Voters   []common.Address `json:"voters"`
	Producer *common.Address  `json:"producer"`
	Password string           `json:"password"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
}
func (args *StartAutoVoteArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	return nil
}

type VoteProducerArgs struct {
	From     common.Address  `json:"from"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Producer common.Address `json:"producer"`
	Amount   *hexutil.Big   `json:"amount"`
}
func (args *VoteProducerArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Amount == nil {
		return errors.New("Amount null")
	}
	return nil
}
func (args *VoteProducerArgs) toTransaction() *types.Transaction {
	return types.NewVoteCreation(&args.Producer, uint64(*args.Nonce), (*big.Int)(args.GasPrice), (*big.Int)(args.Amount))
}
type GetParentArgs struct {
	From common.Address `json:"from"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
}

type SendTextArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Text *string `json:"text"`
}
func (args *SendTextArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.Text == nil || len(*args.Text) <= 0 {
		return errors.New("Empty text!")
	}
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	return nil
}
func (args *SendTextArgs) toTransaction() *types.Transaction {
	if args.To == nil {
		args.To = &args.From
	}
	return types.NewTextCreation(args.To, uint64(*args.Nonce), (*big.Int)(args.GasPrice), []byte(*args.Text))
}
func submitTransaction(ctx context.Context, b Backend, tx *types.Transaction) (common.Hash, error) {
	if err := b.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}
	if tx.To() == nil {
		signer := types.MakeSigner(b.ChainConfig(), b.CurrentBlock().Number())
		from, err := types.Sender(signer, tx)
		if err != nil {
			return common.Hash{}, err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		log.Info("Submitted contract creation", "fullhash", tx.Hash().Hex(), "contract", addr.Hex())
	} else {
		log.Info("Submitted transaction", "fullhash", tx.Hash().Hex(), "recipient", tx.To())
	}
	return tx.Hash(), nil
}

func (s *PublicBlockChainAPI) Voting(ctx context.Context) bool {
	return s.b.IsVoting()
}
func (s *PublicBlockChainAPI) StartStressTesting(ctx context.Context) error {
	gas := params.TxGas
	gasPrice, _ := s.b.SuggestPrice(ctx)
	s.b.StartStressTesting(ctx, gas, gasPrice)
	return nil
}
func (s *PublicBlockChainAPI) StopStressTesting(ctx context.Context) error {
	s.b.StopStressTesting(ctx)
	return nil
}
func (s *PublicBlockChainAPI) StartAutoActive(ctx context.Context, args StartAutoActiveArgs) error {
	if err := args.setDefaults(ctx, s.b); err != nil {
		return err
	}
	s.b.StartAutoActive(ctx, args.Addr, args.Password, (*big.Int)(args.GasPrice))
	return nil
}
func (s *PublicBlockChainAPI) StopAutoActive(ctx context.Context) error {
	s.b.StopAutoActive(ctx)
	return nil
}
func (s *PublicBlockChainAPI) StartAutoVote(ctx context.Context, args StartAutoVoteArgs) error {
	for _, voter := range args.Voters {
		account := accounts.Account{Address: voter}
		err := fetchKeystore(s.b.AccountManager()).Unlock(account, args.Password)
		if err != nil {
			return err
		}
		_, err = s.b.AccountManager().Find(account)
		if err != nil {
			return err
		}
		if err := args.setDefaults(ctx, s.b); err != nil {
			return err
		}
		s.b.AddVoter(voter, args.Password, args.Producer, (*big.Int)(args.GasPrice))
	}
	return nil
}
func (s *PublicBlockChainAPI) StopAutoVote(ctx context.Context) error {
	s.b.CleanVoter()
	return nil
}
func (s *PublicTransactionPoolAPI) VoteProducer(ctx context.Context, args VoteProducerArgs) (common.Hash, error) {
	account := accounts.Account{Address: args.From}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}
	if args.Nonce == nil {
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	tx := args.toTransaction()
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, signed)
}

func (s *PublicTransactionPoolAPI) SendText(ctx context.Context, args SendTextArgs) (common.Hash, error) {
	account := accounts.Account{Address: args.From}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}
	if args.Nonce == nil {
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	tx := args.toTransaction()
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, signed)
}
func (s *PublicTransactionPoolAPI) SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error) {
	account := accounts.Account{Address: args.From}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}
	if args.Nonce == nil {
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	tx := args.toTransaction()
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, signed)
}
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, tx)
}
func (s *PublicTransactionPoolAPI) Sign(addr common.Address, data hexutil.Bytes) (hexutil.Bytes, error) {
	account := accounts.Account{Address: addr}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	signature, err := wallet.SignHash(account, signHash(data))
	if err == nil {
		signature[64] += 27 
	}
	return signature, err
}
type SignTransactionResult struct {
	Raw hexutil.Bytes      `json:"raw"`
	Tx  *types.Transaction `json:"tx"`
}
func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args SendTxArgs) (*SignTransactionResult, error) {
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	tx, err := s.sign(args.From, args.toTransaction())
	if err != nil {
		return nil, err
	}
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	return &SignTransactionResult{data, tx}, nil
}
func (s *PublicTransactionPoolAPI) PendingTransactions() ([]*RPCTransaction, error) {
	pending, err := s.b.GetPoolTransactions()
	if err != nil {
		return nil, err
	}
	transactions := make([]*RPCTransaction, 0, len(pending))
	for _, tx := range pending {
		var signer types.Signer = types.HomesteadSigner{}
		if tx.Protected() {
			signer = types.NewEIP155Signer(tx.ChainId())
		}
		from, _ := types.Sender(signer, tx)
		if _, err := s.b.AccountManager().Find(accounts.Account{Address: from}); err == nil {
			transactions = append(transactions, newRPCPendingTransaction(tx))
		}
	}
	return transactions, nil
}
func (s *PublicTransactionPoolAPI) Resend(ctx context.Context, sendArgs SendTxArgs, gasPrice *hexutil.Big, gasLimit *hexutil.Uint64) (common.Hash, error) {
	if sendArgs.Nonce == nil {
		return common.Hash{}, fmt.Errorf("missing transaction nonce in transaction spec")
	}
	if err := sendArgs.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	matchTx := sendArgs.toTransaction()
	pending, err := s.b.GetPoolTransactions()
	if err != nil {
		return common.Hash{}, err
	}
	for _, p := range pending {
		var signer types.Signer = types.HomesteadSigner{}
		if p.Protected() {
			signer = types.NewEIP155Signer(p.ChainId())
		}
		wantSigHash := signer.Hash(matchTx)
		if pFrom, err := types.Sender(signer, p); err == nil && pFrom == sendArgs.From && signer.Hash(p) == wantSigHash {
			if gasPrice != nil {
				sendArgs.GasPrice = gasPrice
			}
			if gasLimit != nil {
				sendArgs.Gas = gasLimit
			}
			signedTx, err := s.sign(sendArgs.From, sendArgs.toTransaction())
			if err != nil {
				return common.Hash{}, err
			}
			if err = s.b.SendTx(ctx, signedTx); err != nil {
				return common.Hash{}, err
			}
			return signedTx.Hash(), nil
		}
	}
	return common.Hash{}, fmt.Errorf("Transaction %#x not found", matchTx.Hash())
}
type PublicDebugAPI struct {
	b Backend
}
func NewPublicDebugAPI(b Backend) *PublicDebugAPI {
	return &PublicDebugAPI{b: b}
}
func (api *PublicDebugAPI) GetBlockRlp(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	encoded, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", encoded), nil
}
func (api *PublicDebugAPI) PrintBlock(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return block.String(), nil
}
func (api *PublicDebugAPI) SeedHash(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return fmt.Sprintf("0x%x", ethash.SeedHash(number)), nil
}
type PrivateDebugAPI struct {
	b Backend
}
func NewPrivateDebugAPI(b Backend) *PrivateDebugAPI {
	return &PrivateDebugAPI{b: b}
}
func (api *PrivateDebugAPI) ChaindbProperty(property string) (string, error) {
	ldb, ok := api.b.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return "", fmt.Errorf("chaindbProperty does not work for memory databases")
	}
	if property == "" {
		property = "leveldb.stats"
	} else if !strings.HasPrefix(property, "leveldb.") {
		property = "leveldb." + property
	}
	return ldb.LDB().GetProperty(property)
}
func (api *PrivateDebugAPI) ChaindbCompact() error {
	ldb, ok := api.b.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return fmt.Errorf("chaindbCompact does not work for memory databases")
	}
	for b := byte(0); b < 255; b++ {
		log.Info("Compacting chain database", "range", fmt.Sprintf("0x%0.2X-0x%0.2X", b, b+1))
		err := ldb.LDB().CompactRange(util.Range{Start: []byte{b}, Limit: []byte{b + 1}})
		if err != nil {
			log.Error("Database compaction failed", "err", err)
			return err
		}
	}
	return nil
}
func (api *PrivateDebugAPI) SetHead(number hexutil.Uint64) {
	api.b.SetHead(uint64(number))
}
type PublicNetAPI struct {
	net            *p2p.Server
	networkVersion uint64
}
func NewPublicNetAPI(net *p2p.Server, networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{net, networkVersion}
}
func (s *PublicNetAPI) Listening() bool {
	return true 
}
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
	return hexutil.Uint(s.net.PeerCount())
}
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}
