package core
import (
	"fmt"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/params"
	"github.com/DEL-ORG/del/core/state"
)
type BlockValidator struct {
	config *params.ChainConfig 
	bc     *BlockChain         
	engine consensus.Engine    
}
func NewBlockValidator(config *params.ChainConfig, blockchain *BlockChain, engine consensus.Engine) *BlockValidator {
	validator := &BlockValidator{
		config: config,
		engine: engine,
		bc:     blockchain,
	}
	return validator
}
func (v *BlockValidator) ValidateBody(block *types.Block) error {
	if v.bc.HasBlockAndState(block.Hash(), block.NumberU64()) {
		return ErrKnownBlock
	}
	if !v.bc.HasBlockAndState(block.ParentHash(), block.NumberU64()-1) {
		if !v.bc.HasBlock(block.ParentHash(), block.NumberU64()-1) {
			return consensus.ErrUnknownAncestor
		}
		return consensus.ErrPrunedAncestor
	}
	header := block.Header()
	if err := v.engine.VerifyUncles(v.bc, block); err != nil {
		return err
	}
	if hash := types.CalcUncleHash(block.Uncles()); hash != header.UncleHash {
		return fmt.Errorf("uncle root hash mismatch: have %x, want %x", hash, header.UncleHash)
	}
	if hash := types.DeriveSha(block.Transactions()); hash != header.TxHash {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash)
	}
	parent_header := v.bc.GetHeader(header.ParentHash, header.Number.Uint64() - 1)
	if header.Time.Int64() <= parent_header.Time.Int64() {
		return fmt.Errorf("wrong timestamp")
	}
	if err := v.engine.VerifyDifficulty(v.bc, block); err != nil {
		return err
	}
	if err := v.engine.VerifyProducers(v.bc, block); err != nil {
		return err
	}
	if err := v.engine.VerifyVoters(v.bc, block); err != nil {
		return err
	}
	return nil
}
func (v *BlockValidator) ValidateState(block, parent *types.Block, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	header := block.Header()
	if block.GasUsed() != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", block.GasUsed(), usedGas)
	}
	rbloom := types.CreateBloom(receipts)
	if rbloom != header.Bloom {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", header.Bloom, rbloom)
	}
	receiptSha := types.DeriveSha(receipts)
	
	if root := statedb.IntermediateRoot(v.config.IsEIP158(header.Number)); header.Root != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Root, root)
	}
	if receiptSha != header.ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", header.ReceiptHash, receiptSha)
	}
	return nil
}
func CalcGasLimit(parent *types.Block) uint64 {
	contrib := (parent.GasUsed() + parent.GasUsed()/2) / params.GasLimitBoundDivisor
	decay := parent.GasLimit()/params.GasLimitBoundDivisor - 1
	
	limit := parent.GasLimit() - decay + contrib
	if limit < params.MinGasLimit {
		limit = params.MinGasLimit
	}
	if limit < params.TargetGasLimit {
		limit = parent.GasLimit() + decay
		if limit > params.TargetGasLimit {
			limit = params.TargetGasLimit
		}
	}
	return limit
}
