package runtime
import (
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/vm"
)
func NewEnv(cfg *Config) *vm.EVM {
	context := vm.Context{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		CanVote:core.CanVote,
		Vote:core.Vote,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Origin:      cfg.Origin,
		Coinbase:    cfg.Coinbase,
		BlockNumber: cfg.BlockNumber,
		Time:        cfg.Time,
		Difficulty:  cfg.Difficulty,
		GasLimit:    cfg.GasLimit,
		GasPrice:    cfg.GasPrice,
	}
	return vm.NewEVM(context, cfg.State, cfg.ChainConfig, cfg.EVMConfig)
}
