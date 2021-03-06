package core
import (
	"math/big"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/core/vm"
)
type ChainContext interface {
	Engine() consensus.Engine
	GetHeader(common.Hash, uint64) *types.Header
}
func NewEVMContext(msg Message, header *types.Header, chain ChainContext, author *common.Address) vm.Context {
	var beneficiary common.Address
	if author == nil {
		beneficiary, _ = chain.Engine().Author(header) 
	} else {
		beneficiary = *author
	}
	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		CanVote:CanVote,
		Vote:Vote,
		GetHash:     GetHashFn(header, chain),
		Origin:      msg.From(),
		Coinbase:    beneficiary,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        new(big.Int).Set(header.Time),
		Difficulty:  new(big.Int),
		GasLimit:    header.GasLimit,
		GasPrice:    new(big.Int).Set(msg.GasPrice()),
	}
}
func GetHashFn(ref *types.Header, chain ChainContext) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		for header := chain.GetHeader(ref.ParentHash, ref.Number.Uint64()-1); header != nil; header = chain.GetHeader(header.ParentHash, header.Number.Uint64()-1) {
			if header.Number.Uint64() == n {
				return header.Hash()
			}
		}
		return common.Hash{}
	}
}
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}
func CanVote(db vm.StateDB, addr common.Address, totalAmount *big.Int) bool {
	return db.GetBalance(addr).Cmp(totalAmount) >= 0
}
func Vote(db vm.StateDB, sender common.Address, totalAmount *big.Int) {
	db.SubBalance(sender, totalAmount)
	db.AddFreeze(sender, totalAmount)
}
