package core
import (
	"errors"
	"math/big"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core/vm"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/params"
)
var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}
type Message interface {
	From() common.Address
	To() *common.Address
	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int
	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, bool, *big.Int, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}
func (st *StateTransition) from() vm.AccountRef {
	f := st.msg.From()
	if !st.state.Exist(f) {
		st.state.CreateAccount(f)
	}
	return vm.AccountRef(f)
}
func (st *StateTransition) to() vm.AccountRef {
	if st.msg == nil {
		return vm.AccountRef{}
	}
	to := st.msg.To()
	if to == nil {
		return vm.AccountRef{} 
	}
	reference := vm.AccountRef(*to)
	if !st.state.Exist(*to) {
		st.state.CreateAccount(*to)
	}
	return reference
}
func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return vm.ErrOutOfGas
	}
	st.gas -= amount
	return nil
}
func (st *StateTransition) buyGas() error {
	var (
		state  = st.state
		sender = st.from()
	)
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if state.GetBalance(sender.Address()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()
	st.initialGas = st.msg.Gas()
	state.SubBalance(sender.Address(), mgval)
	return nil
}
func (st *StateTransition) preCheck() error {
	msg := st.msg
	sender := st.from()
	if msg.CheckNonce() {
		nonce := st.state.GetNonce(sender.Address())
		if nonce < msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, failed bool, reward *big.Int, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := st.from() 
	contractCreation := msg.To() == nil
	gas, err := params.IntrinsicGas(st.data)
	if err != nil {
		return nil, 0, false, big.NewInt(0), err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, big.NewInt(0), err
	}
	var (
		evm = st.evm
		vmerr error
	)
	if contractCreation {
		ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
	} else {
		st.state.SetNonce(sender.Address(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = evm.Call(sender, st.to().Address(), st.data, st.gas, st.value)
	}
	if vmerr != nil {
		log.Debug("VM returned with error", "err", vmerr)
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, big.NewInt(0), vmerr
		}
	}
	st.refundGas()
	reward = new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
	return ret, st.gasUsed(), vmerr != nil, reward, err
}
func (st *StateTransition) refundGas() {
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund
	sender := st.from()
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(sender.Address(), remaining)
	st.gp.AddGas(st.gas)
}
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
