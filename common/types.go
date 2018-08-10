package common
import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/crypto/sha3"
	"time"
	"github.com/DEL-ORG/del/rlp"
)
const (
	HashLength    = 32
	AddressLength = 20
)
var OFFICIAL = HexToAddress("0x24aa4f9961078c788c397dd27a71b0c5211c2931")
const (
	LEADER_LIMIT = 303
	LEADER_NUMBER = LEADER_LIMIT
	SUPER_COINBASE_RANK = 23
	SLOT_BASE = 5
	MINER_TIMEOUT = SLOT_BASE * 6
	GENESIS_TIME = 1530342720
	RELEASE_NUMBER = 5184
	RELEASE_TIMES = 1000
	BLOCKS_PER_DAY = 17280
)
var	ONE_COIN = new(big.Int).SetUint64(1e18)
var	VOTE_MONEY_LIMIT = new(big.Int).Mul(ONE_COIN, big.NewInt(4096))
const (
	ClientIdentifier = "deld" 
)
func GetTimeBySlot(slot int64) (time.Time) {
	unix := slot * SLOT_BASE
	return time.Unix(unix, 0)
}
func GetCurrentSlotByBigInt(b *big.Int) (int64) {
	if b == nil {
		return 0
	}
	slot := b.Int64() / SLOT_BASE
	return slot
}
func GetCurrentSlot(t time.Time) (int64) {
	tstamp := t.Unix()
	slot := tstamp / SLOT_BASE
	return slot
}
const (
	DataProtocolMessageID_TEXT = 1000
	DataProtocolMessageID_VOTE = 1001
	DataProtocolMessageID_PARENT = 1002
)
const (
	TXTYPE_TRANSFER = "transfer"
	TXTYPE_TEXT = "text"
	TXTYPE_VOTE = "vote"
)
type DataProtocolVote struct {
	Addr string `json:"addr" gencodec:"required"`
	Amount *big.Int `json:"amount,omitempty" gencodec:"required"`
}
type DataProtocolTickets []DataProtocolVote
type DataProtocol struct {
	MessageID uint16 `json:"message_id" gencodec:"required"`
	Text *string `json:"text,omitempty" gencodec:"required"`
	Tickets DataProtocolTickets `json:"tickets,omitempty" gencodec:"required"`
}
func NewDataProtocol(data []byte) (ret *DataProtocol, err error) {
	ret = &DataProtocol{}
	err = rlp.DecodeBytes(data, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
func (self *DataProtocol)Encode() (ret []byte, err error) {
	ret, err = rlp.EncodeToBytes(self)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
func (self DataProtocolTickets)TotalAmount() (total *big.Int) {
	total = big.NewInt(0)
	for _, ticket := range self {
		if ticket.Amount == nil {
			continue
		}
		total.Add(total, ticket.GetAmount())
	}
	return total
}
func (self *DataProtocolVote)GetAmount() *big.Int{
	if self.Amount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set((*big.Int)(self.Amount))
}
func (self *DataProtocol)GetTickets() []DataProtocolVote{
	return self.Tickets
}
func GetRoundNumberByBlockNumber(number uint64) uint64 {
	if number <= 0 {
		return 0
	}
	number = (number - 1) / LEADER_NUMBER
	return number + 1
}
func GetBeginBlockNumberByRoundNumber(round_number uint64) (number uint64) {
	end := GetEndBlockNumberByRoundNumber(round_number)
	if end >= LEADER_NUMBER {
		return end - LEADER_NUMBER + 1
	}else {
		return 0
	}
}
func GetEndBlockNumberByRoundNumber(round_number uint64) (number uint64) {
	return round_number * LEADER_NUMBER
}
var (
	hashT    = reflect.TypeOf(Hash{})
	addressT = reflect.TypeOf(Address{})
)
type Hash [HashLength]byte
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}
func StringToHash(s string) Hash { return BytesToHash([]byte(s)) }
func BigToHash(b *big.Int) Hash  { return BytesToHash(b.Bytes()) }
func HexToHash(s string) Hash    { return BytesToHash(FromHex(s)) }
func (h Hash) Str() string   { return string(h[:]) }
func (h Hash) Bytes() []byte { return h[:] }
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }
func (h Hash) Hex() string   { return hexutil.Encode(h[:]) }
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}
func (h Hash) String() string {
	return h.Hex()
}
func (h Hash) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), h[:])
}
func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}
func (h *Hash) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(hashT, input, h[:])
}
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}
	copy(h[HashLength-len(b):], b)
}
func (h *Hash) SetString(s string) { h.SetBytes([]byte(s)) }
func (h *Hash) Set(other Hash) {
	for i, v := range other {
		h[i] = v
	}
}
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}
func EmptyHash(h Hash) bool {
	return h == Hash{}
}
type UnprefixedHash Hash
func (h *UnprefixedHash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedHash", input, h[:])
}
func (h UnprefixedHash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}
type Address [AddressLength]byte
var oneCoin = big.NewInt(1e18)
var _oneCoin = big.NewFloat(1e18)
func EmptyAddress(address *Address) bool {
	return address == nil || len(*address) <= 0
}
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}
func StringToAddress(s string) Address { return BytesToAddress([]byte(s)) }
func BigToAddress(b *big.Int) Address  { return BytesToAddress(b.Bytes()) }
func HexToAddress(s string) Address    { return BytesToAddress(FromHex(s)) }
func IsHexAddress(s string) bool {
	if hasHexPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressLength && isHex(s)
}
func (a Address) Str() string   { return string(a[:]) }
func (a Address) Bytes() []byte { return a[:] }
func (a Address) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }
func (a Address) Hash() Hash    { return BytesToHash(a[:]) }
func (a Address) Hex() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)
	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}
func (a Address) String() string {
	return a.Hex()
}
func (a Address) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), a[:])
}
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}
func (a *Address) SetString(s string) { a.SetBytes([]byte(s)) }
func (a *Address) Set(other Address) {
	for i, v := range other {
		a[i] = v
	}
}
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}
type UnprefixedAddress Address
func (a *UnprefixedAddress) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedAddress", input, a[:])
}
func (a UnprefixedAddress) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(a[:])), nil
}
