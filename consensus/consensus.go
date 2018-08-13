package consensus
import (
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core/state"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/rpc"
	"math/big"
	"github.com/DEL-ORG/del/accounts"
)
type ChainReader interface {
	Config() *params.ChainConfig
	CurrentHeader() *types.Header
	GetHeader(hash common.Hash, number uint64) *types.Header
	GetHeaderByNumber(number uint64) *types.Header
	GetHeaderByHash(hash common.Hash) *types.Header
	GetBlock(hash common.Hash, number uint64) *types.Block
	GetVoters(header *types.Header) types.Voters
	GetVotersState(header *types.Header) types.VotersMap
	GetGenesisBlock() *types.Block
}
type Engine interface {
	Author(header *types.Header) (common.Address, error)
	VerifyHeader(chain ChainReader, header *types.Header, seal bool) error
	VerifyHeaders(chain ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error)
	VerifyUncles(chain ChainReader, block *types.Block) error
	VerifyDifficulty(chain ChainReader, block *types.Block) error
	VerifyProducers(chain ChainReader, block *types.Block) error
	VerifyVoters(chain ChainReader, block *types.Block) error
	VerifySeal(chain ChainReader, header *types.Header) error
	Prepare(chain ChainReader, header *types.Header, txs types.Transactions) error
	Finalize(chain ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
		uncles []*types.Header, receipts []*types.Receipt, producers types.Producers, voters types.Voters) (*types.Block, error)
	CalProducersWithoutParent(chain ChainReader, header *types.Header) (producers types.Producers, err error)
	CalProducers(chain ChainReader, header *types.Header) (producers types.Producers, err error)
	Seal(chain ChainReader, block *types.Block, state *state.StateDB, am *accounts.Manager, coinbaseDiff *big.Int,stop <-chan struct{}) (*types.Block, error)
	CalcDifficulty(chain ChainReader, header *types.Header, txs types.Transactions) *big.Int
	APIs(chain ChainReader) []rpc.API
}
type PoW interface {
	Engine
	Hashrate() float64
}
