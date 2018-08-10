package core
import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"fmt"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/common/math"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/rlp"
	"sort"
)
var errGenesisNoConfig = errors.New("genesis has no chain configuration")
type Genesis struct {
	Config     *params.ChainConfig `json:"config"`
	Nonce      uint64              `json:"nonce"`
	Timestamp  uint64              `json:"timestamp"`
	ExtraData  []byte              `json:"extraData"`
	GasLimit   uint64              `json:"gasLimit"   gencodec:"required"`
	Difficulty *big.Int            `json:"difficulty" gencodec:"required"`
	Mixhash    common.Hash         `json:"mixHash"`
	Coinbase   common.Address      `json:"coinbase"`
	Alloc      GenesisAlloc        `json:"alloc"      gencodec:"required"`
	Number     uint64      `json:"number"`
	GasUsed    uint64      `json:"gasUsed"`
	ParentHash common.Hash `json:"parentHash"`
}
type GenesisAlloc []GenesisAccount
func (c GenesisAlloc) Len() int {
	return len(c)
}
func (c GenesisAlloc) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
func (c GenesisAlloc) Less(i, j int) bool {
	return c[i].Freeze.Cmp(c[j].Freeze) > 0
}
func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := GenesisAlloc{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	
	return nil
}
type GenesisAccount struct {
	Addr       common.Address              `json:"addr" gencodec:"required"`
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Freeze     *big.Int                    `json:"freeze" gencodec:"required"`
	Producer   bool                        `json:"producer" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` 
}
type genesisSpecMarshaling struct {
	Nonce      math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
	ExtraData  hexutil.Bytes
	GasLimit   math.HexOrDecimal64
	GasUsed    math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Difficulty *math.HexOrDecimal256
	Alloc      GenesisAlloc
}
type genesisAccountMarshaling struct {
	Code       hexutil.Bytes
	Balance    *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	Storage    map[storageJSON]storageJSON
	PrivateKey hexutil.Bytes
}
type storageJSON common.Hash
func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}
func (h storageJSON) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}
type GenesisMismatchError struct {
	Stored, New common.Hash
}
func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}
func SetupGenesisBlock(db ethdb.Database, genesis *Genesis) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.AllEthashProtocolChanges, common.Hash{}, errGenesisNoConfig
	}
	stored := GetCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		return genesis.Config, block.Hash(), err
	}
	if genesis != nil {
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}
	newcfg := genesis.configOrDefault(stored)
	storedcfg, err := GetChainConfig(db, stored)
	if err != nil {
		if err == ErrChainConfigNotFound {
			log.Warn("Found genesis block without chain config")
			err = WriteChainConfig(db, stored, newcfg)
		}
		return newcfg, stored, err
	}
	
	height := GetBlockNumber(db, GetHeadHeaderHash(db))
	if height == missingNumber {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, height)
	if compatErr != nil && height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	return newcfg, stored, WriteChainConfig(db, stored, newcfg)
}
func (g *Genesis) configOrDefault(ghash common.Hash) *params.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	default:
		return params.MainnetChainConfig
	}
}
func (g *Genesis) ToBlock(db ethdb.Database) *types.Block {
	if db == nil {
		db, _ = ethdb.NewMemDatabase()
	}
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	producers := types.Producers{}
	
	for _, account := range g.Alloc { 
		addr := account.Addr
		statedb.AddBalance(addr, account.Balance)
		statedb.AddFreeze(addr, account.Freeze)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
		if account.Producer {
			producers = append(producers, types.Producer{Addr: addr, Vote: new(big.Int).SetInt64(0)})
		}
	}
	if len(producers) > common.LEADER_LIMIT {
		log.Error("Genesis producers error", "len", len(producers))
		producers = producers[:common.LEADER_LIMIT]
	}
	sort.Stable(producers)
	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		Number:     new(big.Int).SetUint64(g.Number),
		Nonce:      types.EncodeNonce(g.Nonce),
		Time:       new(big.Int).SetUint64(g.Timestamp),
		ParentHash: g.ParentHash,
		Extra:      g.ExtraData,
		GasLimit:   g.GasLimit,
		GasUsed:    g.GasUsed,
		Difficulty: g.Difficulty,
		MixDigest:  g.Mixhash,
		Coinbase:   g.Coinbase,
		Root:       root,
	}
	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)
	return types.NewBlock(head, nil, nil, nil, producers, nil)
}
func (g *Genesis) Commit(db ethdb.Database) (*types.Block, error) {
	block := g.ToBlock(db)
	if block.Number().Sign() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with number > 0")
	}
	if err := WriteTd(db, block.Hash(), block.NumberU64(), g.Difficulty); err != nil {
		return nil, err
	}
	if err := WriteBlock(db, block); err != nil {
		return nil, err
	}
	if err := WriteBlockReceipts(db, block.Hash(), block.NumberU64(), nil); err != nil {
		return nil, err
	}
	if err := WriteCanonicalHash(db, block.Hash(), block.NumberU64()); err != nil {
		return nil, err
	}
	if err := WriteHeadBlockHash(db, block.Hash()); err != nil {
		return nil, err
	}
	if err := WriteHeadHeaderHash(db, block.Hash()); err != nil {
		return nil, err
	}
	config := g.Config
	if config == nil {
		config = params.AllEthashProtocolChanges
	}
	return block, WriteChainConfig(db, block.Hash(), config)
}
func (g *Genesis) MustCommit(db ethdb.Database) *types.Block {
	block, err := g.Commit(db)
	if err != nil {
		panic(err)
	}
	return block
}
func GenesisBlockForTesting(db ethdb.Database, addr common.Address, balance *big.Int) *types.Block {
	g := Genesis{Alloc: GenesisAlloc{{Addr: addr, Balance: balance}}}
	return g.MustCommit(db)
}
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.MainnetChainConfig,
		Nonce:      88,
		Timestamp:  common.GENESIS_TIME,
		ExtraData:  hexutil.MustDecode("0xE68891E7A9BAE80000"),
		GasLimit:   params.MinGasLimit,
		Difficulty: big.NewInt(0),
		Coinbase:   common.HexToAddress("0x00a40567f14ce62c826a856ebcc004fe43d10360"),
		Alloc:      decodePrealloc(mainnetAllocData),
	}
}
func DefaultTestnetGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.TestnetChainConfig,
		Nonce:      88,
		Timestamp:  common.GENESIS_TIME,
		ExtraData:  hexutil.MustDecode("0xE68891E7A9BAE80000"),
		GasLimit:   params.MinGasLimit,
		Difficulty: big.NewInt(0),
		Coinbase:   common.HexToAddress("0x000fa0b5f9e519af8296b7406bf632bc9ec72cee"),
		Alloc:      decodePrealloc(testnetAllocData),
	}
}
func DefaultRinkebyGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.RinkebyChainConfig,
		Timestamp:  common.GENESIS_TIME,
		ExtraData:  hexutil.MustDecode("0x52657370656374206d7920617574686f7269746168207e452e436172746d616e42eb768f2244c8811c63729a21a3569731535f067ffc57839b00206d1ad20c69a1981b489f772031b279182d99e65703f0076e4812653aab85fca0f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		GasLimit:   4700000,
		Difficulty: big.NewInt(1),
		Alloc:      decodePrealloc(rinkebyAllocData),
	}
}
func DeveloperGenesisBlock(period uint64, faucet common.Address) *Genesis {
	config := *params.AllCliqueProtocolChanges
	config.Clique.Period = period
	return &Genesis{
		Config:     &config,
		ExtraData:  append(append(make([]byte, 32), faucet[:]...), make([]byte, 65)...),
		GasLimit:   6283185,
		Difficulty: big.NewInt(1),
		Alloc: []GenesisAccount{
			{Addr: common.BytesToAddress([]byte{1}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{2}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{3}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{4}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{5}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{6}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{7}), Balance: big.NewInt(1)}, 
			{Addr: common.BytesToAddress([]byte{8}), Balance: big.NewInt(1)}, 
			{Addr: faucet, Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(9))},
		},
	}
}
func decodePrealloc(data string) GenesisAlloc {
	var p []struct {
		Addr            common.Address
		Balance, Freeze *big.Int
		Producer        bool
	}
	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := GenesisAlloc{}
	for _, account := range p {
		ga = append(ga, GenesisAccount{Balance: account.Balance, Freeze: account.Freeze, Addr: account.Addr, Producer: account.Producer})
	}
	if !sort.IsSorted(ga) {
		log.Error("Genesis alloc not sorted.")
	}
	return ga
}
