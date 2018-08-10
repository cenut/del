package core
import (
	"fmt"
	"math/big"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/consensus/misc"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core/vm"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/params"
)
var (
	canonicalSeed = 1
	forkSeed      = 2
)
type BlockGen struct {
	i           int
	parent      *types.Block
	chain       []*types.Block
	chainReader consensus.ChainReader
	header      *types.Header
	statedb     *state.StateDB
	gasPool  *GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt
	uncles   []*types.Header
	producers   types.Producers
	voters   types.Voters
	config *params.ChainConfig
	engine consensus.Engine
}
func (b *BlockGen) SetCoinbase(addr common.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("coinbase must be set before adding transactions")
		}
		panic("coinbase can only be set once")
	}
	b.header.Coinbase = addr
	b.gasPool = new(GasPool).AddGas(b.header.GasLimit)
}
func (b *BlockGen) SetExtra(data []byte) {
	b.header.Extra = data
}
func (b *BlockGen) SetProducers(producers types.Producers) types.Producers {
	if producers == nil {
		b.producers = b.parent.Producers()
	}else {
		b.producers = producers
	}
	return b.producers
}
func (b *BlockGen) AddTx(tx *types.Transaction) {
	if b.gasPool == nil {
		b.SetCoinbase(common.Address{})
	}
	b.statedb.Prepare(tx.Hash(), common.Hash{}, len(b.txs))
	receipt, _, _, err := ApplyTransaction(b.config, nil, &b.header.Coinbase, b.gasPool, b.statedb, b.header, tx, &b.header.GasUsed, vm.Config{})
	if err != nil {
		panic(err)
	}
	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}
func (b *BlockGen) Number() *big.Int {
	return new(big.Int).Set(b.header.Number)
}
func (b *BlockGen) AddUncheckedReceipt(receipt *types.Receipt) {
	b.receipts = append(b.receipts, receipt)
}
func (b *BlockGen) TxNonce(addr common.Address) uint64 {
	if !b.statedb.Exist(addr) {
		panic("account does not exist")
	}
	return b.statedb.GetNonce(addr)
}
func (b *BlockGen) AddUncle(h *types.Header) {
	b.uncles = append(b.uncles, h)
}
func (b *BlockGen) PrevBlock(index int) *types.Block {
	if index >= b.i {
		panic("block index out of range")
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}
func (b *BlockGen) OffsetTime(seconds int64) {
	b.header.Time.Add(b.header.Time, new(big.Int).SetInt64(seconds))
	if b.header.Time.Cmp(b.parent.Header().Time) <= 0 {
		panic("block time out of range")
	}
	b.header.Difficulty = b.engine.CalcDifficulty(b.chainReader, b.header, nil)
}
func (b *BlockGen) GetTime() *big.Int {
	return b.header.Time
}
func GenerateChain(config *params.ChainConfig, parent *types.Block, engine consensus.Engine, db ethdb.Database, n int, gen func(int, *BlockGen)) ([]*types.Block, []types.Receipts) {
	if config == nil {
		config = params.TestChainConfig
	}
	blocks, receipts := make(types.Blocks, n), make([]types.Receipts, n)
	genblock := func(i int, parent *types.Block, statedb *state.StateDB) (*types.Block, types.Receipts) {
		blockchain, _ := NewBlockChain(db, nil, config, engine, vm.Config{})
		defer blockchain.Stop()
		b := &BlockGen{i: i, parent: parent, chain: blocks, chainReader: blockchain, statedb: statedb, config: config, engine: engine}
		b.header = makeHeader(b.chainReader, parent, statedb, b.engine)
		if daoBlock := config.DAOForkBlock; daoBlock != nil {
			limit := new(big.Int).Add(daoBlock, params.DAOForkExtraRange)
			if b.header.Number.Cmp(daoBlock) >= 0 && b.header.Number.Cmp(limit) < 0 {
				if config.DAOForkSupport {
					b.header.Extra = common.CopyBytes(params.DAOForkBlockExtra)
				}
			}
		}
		if config.DAOForkSupport && config.DAOForkBlock != nil && config.DAOForkBlock.Cmp(b.header.Number) == 0 {
			misc.ApplyDAOHardFork(statedb)
		}
		if gen != nil {
			gen(i, b)
		}
		if b.engine != nil {
			block, _ := b.engine.Finalize(b.chainReader, b.header, statedb, b.txs, b.uncles, b.receipts, b.producers, b.voters)
			root, err := statedb.Commit(config.IsEIP158(b.header.Number))
			if err != nil {
				panic(fmt.Sprintf("state write error: %v", err))
			}
			if err := statedb.Database().TrieDB().Commit(root, false); err != nil {
				panic(fmt.Sprintf("trie write error: %v", err))
			}
			return block, b.receipts
		}
		return nil, nil
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}
	return blocks, receipts
}
func GenerateChainAndSave(config *params.ChainConfig, parent *types.Block, engine consensus.Engine, db ethdb.Database, n int,
	gen func(int, *BlockGen),
	onBlock func(*types.Block, *types.Block)) ([]*types.Block, []types.Receipts) {
	if config == nil {
		config = params.TestChainConfig
	}
	blocks, receipts := make(types.Blocks, n), make([]types.Receipts, n)
	genblock := func(i int, parent *types.Block, statedb *state.StateDB) (*types.Block, types.Receipts) {
		blockchain, _ := NewBlockChain(db, nil, config, engine, vm.Config{})
		defer blockchain.Stop()
		b := &BlockGen{i: i, parent: parent, chain: blocks, chainReader: blockchain, statedb: statedb, config: config, engine: engine}
		b.header = makeHeader(b.chainReader, parent, statedb, b.engine)
		if daoBlock := config.DAOForkBlock; daoBlock != nil {
			limit := new(big.Int).Add(daoBlock, params.DAOForkExtraRange)
			if b.header.Number.Cmp(daoBlock) >= 0 && b.header.Number.Cmp(limit) < 0 {
				if config.DAOForkSupport {
					b.header.Extra = common.CopyBytes(params.DAOForkBlockExtra)
				}
			}
		}
		if config.DAOForkSupport && config.DAOForkBlock != nil && config.DAOForkBlock.Cmp(b.header.Number) == 0 {
			misc.ApplyDAOHardFork(statedb)
		}
		if gen != nil {
			gen(i, b)
		}
		if b.engine != nil {
			block, _ := b.engine.Finalize(b.chainReader, b.header, statedb, b.txs, b.uncles, b.receipts, b.producers, b.voters)
			root, err := statedb.Commit(config.IsEIP158(b.header.Number))
			if err != nil {
				panic(fmt.Sprintf("state write error: %v", err))
			}
			if err := statedb.Database().TrieDB().Commit(root, false); err != nil {
				panic(fmt.Sprintf("trie write error: %v", err))
			}
			return block, b.receipts
		}
		return nil, nil
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		if i <= 0 {
			onBlock(parent, block)
		}else {
			onBlock(blocks[i - 1], block)
		}
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}
	return blocks, receipts
}
func makeHeader(chain consensus.ChainReader, parent *types.Block, state *state.StateDB, engine consensus.Engine) *types.Header {
	var time *big.Int
	if parent.Time() == nil {
		time = big.NewInt(common.SLOT_BASE)
	} else {
		time = new(big.Int).Add(parent.Time(), big.NewInt(common.SLOT_BASE)) 
	}
	return &types.Header{
		Root:       state.IntermediateRoot(chain.Config().IsEIP158(parent.Number())),
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		Difficulty: engine.CalcDifficulty(chain, &types.Header{
			Number:     parent.Number(),
			Time:       new(big.Int).Sub(time, big.NewInt(10)),
			Difficulty: parent.Difficulty(),
			UncleHash:  parent.UncleHash(),
		}, nil),
		GasLimit: CalcGasLimit(parent),
		Number:   new(big.Int).Add(parent.Number(), common.Big1),
		Time:     parent.Time(),
	}
}
func newCanonical(engine consensus.Engine, n int, full bool) (ethdb.Database, *BlockChain, error) {
	gspec := new(Genesis)
	db, _ := ethdb.NewMemDatabase()
	genesis := gspec.MustCommit(db)
	blockchain, _ := NewBlockChain(db, nil, params.AllEthashProtocolChanges, engine, vm.Config{})
	if n == 0 {
		return db, blockchain, nil
	}
	if full {
		blocks := makeBlockChain(genesis, n, engine, db, canonicalSeed)
		_, err := blockchain.InsertChain(blocks)
		return db, blockchain, err
	}
	headers := makeHeaderChain(genesis.Header(), n, engine, db, canonicalSeed)
	_, err := blockchain.InsertHeaderChain(headers, 1)
	return db, blockchain, err
}
func makeHeaderChain(parent *types.Header, n int, engine consensus.Engine, db ethdb.Database, seed int) []*types.Header {
	blocks := makeBlockChain(types.NewBlockWithHeader(parent), n, engine, db, seed)
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	return headers
}
func makeBlockChain(parent *types.Block, n int, engine consensus.Engine, db ethdb.Database, seed int) []*types.Block {
	blocks, _ := GenerateChain(params.TestChainConfig, parent, engine, db, n, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{0: byte(seed), 19: byte(i)})
	})
	return blocks
}
