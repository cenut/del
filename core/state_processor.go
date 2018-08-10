package core
import (
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core/vm"
	"github.com/DEL-ORG/del/crypto"
	"github.com/DEL-ORG/del/params"
	"math/big"
	"github.com/DEL-ORG/del/log"
)
type StateProcessor struct {
	config *params.ChainConfig 
	bc     *BlockChain         
	engine consensus.Engine    
}
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}
func ApplyReleaseGenesisBalance(chain consensus.ChainReader, header *types.Header, state *state.StateDB) {
	if header.Number.Uint64() <= 0 {
		return
	}
	if header.Number.Uint64() % common.RELEASE_NUMBER != 0 {
		return
	}
	genesis := DefaultGenesisBlock()
	if chain.Config().ChainId == DefaultTestnetGenesisBlock().Config.ChainId {
		genesis = DefaultTestnetGenesisBlock()
	}
	if header.Number.Uint64() / common.RELEASE_NUMBER <= common.RELEASE_TIMES {
		for _, account := range genesis.Alloc {
			if account.Freeze.Cmp(common.Big0) <= 0 {
				break
			}
			freeze := new(big.Int).Set(account.Freeze)
			freeze.Div(freeze, new(big.Int).SetInt64(common.RELEASE_TIMES))
			state.AddBalance(account.Addr, freeze)
			state.SubFreeze(account.Addr, freeze)
		}
	}else if (header.Number.Uint64() / common.RELEASE_NUMBER) == (common.RELEASE_TIMES + 1) {
		for _, account := range genesis.Alloc {
			if account.Freeze.Cmp(common.Big0) <= 0 {
				break
			}
			remain := new(big.Int).Set(account.Freeze)
			freeze := new(big.Int).Set(account.Freeze)
			freeze.Div(freeze, new(big.Int).SetInt64(common.RELEASE_TIMES))
			freeze.Mul(freeze, big.NewInt(common.RELEASE_TIMES))
			remain.Sub(remain, freeze)
			if remain.Cmp(common.Big0) > 0 {
				state.AddBalance(account.Addr, remain)
			}
		}
	}
}
func ApplyReleaseVoterBalance(chain consensus.ChainReader, header *types.Header, state *state.StateDB, transactions types.Transactions) {
	current_round_number := common.GetRoundNumberByBlockNumber(header.Number.Uint64())
	current_round_end_block_number := common.GetEndBlockNumberByRoundNumber(current_round_number)
	if header.Number.Uint64() != current_round_end_block_number {
		return
	}
	current_round_begin_block_number := common.GetBeginBlockNumberByRoundNumber(current_round_number)
	process := func(signer types.Signer, txs types.Transactions) {
		for _, tx := range txs {
			message, err  := tx.GetMessage()
			if err != nil || message == nil {
				continue
			}
			if message.MessageID == common.DataProtocolMessageID_VOTE {
				sender, err := types.Sender(signer, tx)
				if err == nil {
					totalAmount := message.Tickets.TotalAmount()
					state.SubFreeze(sender, totalAmount)
					state.AddBalance(sender, totalAmount)
				}
			}
		}
	}
	if transactions != nil {
		signer := types.MakeSigner(chain.Config(), header.Number)
		process(signer, transactions)
	}
	for header = chain.GetHeader(header.ParentHash, header.Number.Uint64() - 1);
		header != nil && header.Number.Uint64() >= current_round_begin_block_number;
		header = chain.GetHeader(header.ParentHash, header.Number.Uint64() - 1) {
		block := chain.GetBlock(header.Hash(), header.Number.Uint64())
		if block == nil {
			log.Error("Can not load block", "hash", header.Hash())
		}
		signer := types.MakeSigner(chain.Config(), header.Number)
		process(signer, block.Transactions())
	}
}
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
	
	ApplyReleaseGenesisBalance(p.bc, block.Header(), statedb)
	totalReward := big.NewInt(0)
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, reward, err := ApplyTransaction(p.config, p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		totalReward.Add(totalReward, reward)
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	statedb.AddBalance(block.Coinbase(), totalReward)
	ApplyReleaseVoterBalance(p.bc, block.Header(), statedb, block.Transactions())
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles(), receipts, block.Producers(), block.Voters)
	return receipts, allLogs, *usedGas, nil
}
func ApplyTransaction(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, *big.Int, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, big.NewInt(0), err
	}
	context := NewEVMContext(msg, header, bc, author)
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	_, gas, failed, reward, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, big.NewInt(0), err
	}
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	return receipt, gas, reward, err
}
