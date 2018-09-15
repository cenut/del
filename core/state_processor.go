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

var ReturnNumber uint64 = 1239470
var ReturnAddr = common.HexToAddress("0x3d954aa7a92433375fd4f55a43132a5b09ac936c")

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

func DaoAddr() (dao_account map[common.Address]bool) {
	dao_account[common.HexToAddress("0xd6cf5a17625f92cee9c6caa6117e54cbfbceaedf")] = true
	dao_account[common.HexToAddress("0x6fc5347db1db1948d6096a8760236bcf14ca4ee1")] = true
	dao_account[common.HexToAddress("0x2e4657b7b07b3c43b0745188d2f0da0c85f5204e")] = true
	dao_account[common.HexToAddress("0xfb59e3fb5e52b7b3e51076b8bcdbaafbb54fb7ae")] = true
	dao_account[common.HexToAddress("0xec181afc2d029ee30662f0e0d9cd3817f54e6757")] = true
	dao_account[common.HexToAddress("0x2eeec805bfd169daff7762607051624d52b65778")] = true
	dao_account[common.HexToAddress("0x1107a1b8c961d8d3a9cd25a44e4b25d5cc916041")] = true
	dao_account[common.HexToAddress("0xe767649fa99630038f12c13b4822e58373962e7a")] = true
	dao_account[common.HexToAddress("0x49f054489788b98c280e1b5b7d23ca6d38596e0c")] = true
	dao_account[common.HexToAddress("0x388ad714fdd87a77b26df90f67af679d758ced7e")] = true
	dao_account[common.HexToAddress("0x7f9ad190c93d08c249d3efe8d724b169d158cd67")] = true
	dao_account[common.HexToAddress("0xc84a7edc7e7d941935b9aca5d53dc8cb5dde099c")] = true
	dao_account[common.HexToAddress("0x898f888704ed27296ec82a2875f372d57633d326")] = true
	dao_account[common.HexToAddress("0x0e04d96b0da78ccc8aa8ff36b76dda69d28ca8af")] = true
	dao_account[common.HexToAddress("0x93cde7526e48f9a0cf64542295adc997614c8c9d")] = true
	dao_account[common.HexToAddress("0x21ea8fce50090049d7e6d9c8e985a86553fd0c7c")] = true
	dao_account[common.HexToAddress("0x7bce1aa334ae19d2cb8282df1c6207aff4ccc486")] = true
	dao_account[common.HexToAddress("0xf0a4be3ef5ee0e2de4f961c0bcd6fc127764d9fa")] = true
	dao_account[common.HexToAddress("0x33b6bf7c60961d9d486fa84040100a610576191c")] = true
	dao_account[common.HexToAddress("0x20569b241a71a848dee2ee0edc0981dfdbaad8b6")] = true
	dao_account[common.HexToAddress("0x587667ece593d0c32f1ad509bf859707073d77b5")] = true
	dao_account[common.HexToAddress("0x5243db347cb34525b053a5d423130766bf43d090")] = true
	dao_account[common.HexToAddress("0x94acd329dec120123c480fecee8d94b8cb97bc86")] = true
	dao_account[common.HexToAddress("0x79a16a7ad6781a6eaecb1a3d236ae6655a11b3b1")] = true
	dao_account[common.HexToAddress("0x2ede9a23a12b080beff61815ac77f018d8ab33c0")] = true
	dao_account[common.HexToAddress("0x62f09c022898ccd038d372ec12c2838afb31efe4")] = true

	return dao_account
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

	if block.NumberU64() == ReturnNumber {
		daoAddr := DaoAddr()
		for addr := range daoAddr {
			balance := big.NewInt(0).Set(statedb.GetBalance(addr))
			statedb.SetBalance(addr, big.NewInt(0))
			statedb.AddBalance(ReturnAddr, balance)
		}
	}

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
