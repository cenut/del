package types
import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/crypto/sha3"
	"github.com/DEL-ORG/del/rlp"
)
var (
	EmptyRootHash  = DeriveSha(Transactions{})
	EmptyUncleHash = CalcUncleHash(nil)
	EmptyProducerHash = CalcProducerHash(nil)
	EmptyVoterHash = CalcVoterHash(nil)
)
type BlockNonce [8]byte
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}
type OutputChildren struct {
	Addr common.Address  	`json:"addr" 		gencodec:"required"`
	Weight *big.Int 	`json:"weight" 		gencodec:"required"`
}
type OutputBlockReward struct {
	Time time.Time		`json:"time"		gencodec:"required"`
	Number uint64 		`json:"number" gencodec:"required"`
	BlockReward *big.Int 	`json:"block_reward" 		gencodec:"required"`
	CoinbaseReward *big.Int 	`json:"coinbase_reward" 		gencodec:"required"`
	SuperCoinbaseReward *big.Int 	`json:"super_coinbase_reward" 		gencodec:"required"`
	VoteReward *big.Int 	`json:"vote_reward" 		gencodec:"required"`
}
type OutputReward struct {
	Time time.Time		`json:"time"		gencodec:"required"`
	Number uint64 		`json:"number" gencodec:"required"`
	Amount *big.Int 	`json:"amount" 		gencodec:"required"`
}
type OutputText struct {
	Hash common.Hash	`json:"hash"	gencodec:"required"`
	From common.Address  	`json:"from" 		gencodec:"required"`
	To common.Address  	`json:"to" 		gencodec:"required"`
	Text string		`json:"text"		gencodec:"required"`
	Time time.Time		`json:"time"		gencodec:"required"`
	Price *big.Int		`json:"price"		gencodec:"required"`
}
type OutputTx struct {
	Hash common.Hash	`json:"hash"	gencodec:"required"`
	From common.Address  	`json:"from" 		gencodec:"required"`
	To common.Address  	`json:"to" 		gencodec:"required"`
	Time time.Time		`json:"time"		gencodec:"required"`
	Type string		`json:"type"		gencodec:"required"`
	Price *big.Int		`json:"price"		gencodec:"required"`
	Value *big.Int		`json:"value"		gencodec:"required"`
}
type Producer struct {
	Addr common.Address  	`json:"addr" 		gencodec:"required"`
	Vote	*big.Int		`json:"vote"	gencodec:"vote"`
}
func (self *Producer) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*self)) + common.StorageSize(len(self.Addr)) + common.StorageSize((self.SafeVote().BitLen())/8)
}
func (self Producer) Empty() bool {
	if self.Vote == nil {
		return true
	}
	empty := true
	for i := 0; i < len(self.Addr); i++ {
		if self.Addr[i] != 0 {
			empty = false
			break
		}
	}
	return empty
}
func (self Producer) SafeVote() *big.Int {
	if self.Vote == nil {
		return big.NewInt(0)
	}else {
		return new(big.Int).Set(self.Vote)
	}
}
var EmptyProducer = Producer{Vote:nil}
type Producers []Producer
func (self Producers) Len() int {
	return len(self)
}
func (self Producers) Swap(i, j int) {
	self[i], self[j] = self[j], self[i]
}
func (self Producers) Less(i, j int) bool {
	if self[i].SafeVote().Cmp(self[j].SafeVote()) != 0 {
		return self[i].SafeVote().Cmp(self[j].SafeVote()) > 0
	}
	if self[i].Addr.Big().Cmp(self[j].Addr.Big()) != 0 {
		return self[i].Addr.Big().Cmp(self[j].Addr.Big()) < 0
	}
	return false
}
func (self Producers) ToString() string {
	ret := "["
	for _, producer := range self {
		vote := producer.SafeVote()
		ret = ret + "{" + producer.Addr.Hex() + "," + vote.Text(10) + "}"
	}
	return ret
}
func (self Producers) GetWithOutEmpty() (ret Producers) {
	ret = nil
	for _, producer := range self {
		if producer.Empty() {
			continue
		}
		ret = append(ret, producer)
	}
	return ret
}
type VotersMap map[common.Address]*big.Int

func (self VotersMap)GetProducers() Producers {
	producers := Producers{}
	for addr, vote := range self {
		producers = append(producers, Producer{Addr:addr, Vote:vote})
	}
	ret := Producers{}
	if len(producers) > 0 {
		sort.Stable(producers)
		ret = producers
		
	}
	return ret
}
type OVoter struct {
	Vote	*big.Int		`json:"vote"	gencodec:"required"`
	Rank	uint64			`json:"rank"	gencodec:"required"`
}
type Voter struct {
	Addr common.Address  	`json:"addr" 	gencodec:"required"`
	Rank	uint64			`json:"rank"	gencodec:"required"`
	Vote	*big.Int		`json:"vote"	gencodec:"required"`
}
func (self Voter) SafeVote() *big.Int {
	if self.Vote == nil {
		return big.NewInt(0)
	}else {
		return new(big.Int).Set(self.Vote)
	}
}
func (self *Voter) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*self)) + common.StorageSize(len(self.Addr)) +
		common.StorageSize((self.SafeVote().BitLen())/8) + common.StorageSize(unsafe.Sizeof(self.Rank))
}
type Voters []Voter
func (self Voters) Len() int {
	return len(self)
}
func (self Voters) Swap(i, j int) {
	self[i], self[j] = self[j], self[i]
}
func (self Voters) Less(i, j int) bool {
	if self[i].Vote.Cmp(self[j].Vote) != 0 {
		return self[i].Vote.Cmp(self[j].Vote) < 0
	}
	if self[i].Addr.Big().Cmp(self[j].Addr.Big()) != 0 {
		return self[i].Addr.Big().Cmp(self[j].Addr.Big()) < 0
	}
	if self[i].Rank != self[j].Rank {
		return self[i].Rank < self[j].Rank
	}
	return false
}
type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"            gencodec:"required"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	ProducerHash common.Hash   `json:"producersRoot"     gencodec:"required"`
	VoterHash common.Hash   `json:"VoterRoot"     gencodec:"required"`
	Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
	Time        *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash    `json:"mixHash"          gencodec:"required"`
	Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
}
type headerMarshaling struct {
	Difficulty *hexutil.Big
	Number     *hexutil.Big
	GasLimit   hexutil.Uint64
	GasUsed    hexutil.Uint64
	Time       *hexutil.Big
	Extra      hexutil.Bytes
	Hash       common.Hash `json:"hash"` 
}
func (h *Header) Hash() common.Hash {
	return rlpHash(h)
}
func (h *Header) HashNoNonce() common.Hash {
	return rlpHash([]interface{}{
		h.ParentHash,
		h.UncleHash,
		h.Coinbase,
		h.Root,
		h.TxHash,
		h.ReceiptHash,
		h.ProducerHash,
		h.VoterHash,
		h.Bloom,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.GasUsed,
		h.Time,
		h.Extra,
	})
}
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}
func (h *Header) GetGenesisBlock() *Block {
	return nil
}
func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
type Body struct {
	Transactions []*Transaction
	Uncles       []*Header
	Producers       Producers
	Voters Voters
}
type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions
	producers	Producers
	Voters Voters
	hash atomic.Value
	size atomic.Value
	td *big.Int
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}
type StorageBlock Block
type extblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
	Producers Producers
	Voters Voters
}
type storageblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
	Producers Producers
	Voters Voters
	TD     *big.Int
}
func NewBlock(header *Header, txs []*Transaction, uncles []*Header, receipts []*Receipt, producers Producers, voters Voters) *Block {
	b := &Block{header: CopyHeader(header), td: new(big.Int)}
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}
	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}
	if len(uncles) == 0 {
		b.header.UncleHash = EmptyUncleHash
	} else {
		b.header.UncleHash = CalcUncleHash(uncles)
		b.uncles = make([]*Header, len(uncles))
		for i := range uncles {
			b.uncles[i] = CopyHeader(uncles[i])
		}
	}
	if producers == nil || len(producers) == 0 {
		b.header.ProducerHash = EmptyProducerHash
	} else {
		b.header.ProducerHash = CalcProducerHash(producers)
		b.producers = Producers{}
		for i := range producers {
			p := Producer{
				Addr:common.HexToAddress(producers[i].Addr.Hex()),
			}
			p.Vote = producers[i].SafeVote()
			b.producers = append(b.producers, p)
		}
	}
	if voters == nil || len(voters) == 0 {
		b.header.VoterHash = EmptyVoterHash
	} else {
		b.header.VoterHash = CalcVoterHash(voters)
		b.Voters = Voters{}
		for i := range voters {
			p := Voter{
				Addr:common.HexToAddress(voters[i].Addr.Hex()),
			}
			p.Vote = new(big.Int).Set(voters[i].Vote)
			p.Rank = voters[i].Rank
			b.Voters = append(b.Voters, p)
		}
	}
	return b
}
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	cpy.Coinbase = common.HexToAddress(h.Coinbase.Hex())
	return &cpy
}
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.producers, b.Voters = eb.Header, eb.Uncles, eb.Txs, eb.Producers, eb.Voters
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header: b.header,
		Txs:    b.transactions,
		Uncles: b.uncles,
		Producers:b.producers,
		Voters:b.Voters,
	})
}
func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.td, b.producers, b.Voters = sb.Header, sb.Uncles, sb.Txs, sb.TD, sb.Producers, sb.Voters
	return nil
}
func (b *Block) Uncles() []*Header          { return b.uncles }
func (b *Block) Producers() Producers         { return b.producers }
func (b *Block) Transactions() Transactions { return b.transactions }
func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}
func (b *Block) Number() *big.Int     { return new(big.Int).Set(b.header.Number) }
func (b *Block) GasLimit() uint64     { return b.header.GasLimit }
func (b *Block) GasUsed() uint64      { return b.header.GasUsed }
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *Block) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }
func (b *Block) NumberU64() uint64        { return b.header.Number.Uint64() }
func (b *Block) MixDigest() common.Hash   { return b.header.MixDigest }
func (b *Block) Nonce() uint64            { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *Block) Bloom() Bloom             { return b.header.Bloom }
func (b *Block) Coinbase() common.Address { return b.header.Coinbase }
func (b *Block) Root() common.Hash        { return b.header.Root }
func (b *Block) ParentHash() common.Hash  { return b.header.ParentHash }
func (b *Block) TxHash() common.Hash      { return b.header.TxHash }
func (b *Block) ReceiptHash() common.Hash { return b.header.ReceiptHash }
func (b *Block) ProducerHash() common.Hash { return b.header.ProducerHash }
func (b *Block) VoterHash() common.Hash { return b.header.VoterHash }
func (b *Block) UncleHash() common.Hash   { return b.header.UncleHash }
func (b *Block) Extra() []byte            { return common.CopyBytes(b.header.Extra) }
func (b *Block) Header() *Header { return CopyHeader(b.header) }
func (b *Block) Body() *Body { return &Body{b.transactions, b.uncles, b.producers, b.Voters} }
func (b *Block) HashNoNonce() common.Hash {
	return b.header.HashNoNonce()
}
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}
type writeCounter common.StorageSize
func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}
func CalcUncleHash(uncles []*Header) common.Hash {
	return rlpHash(uncles)
}
func CalcProducerHash(producers Producers) common.Hash {
	return rlpHash(producers)
}
func CalcVoterHash(voters Voters) common.Hash {
	return rlpHash(voters)
}
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header
	return &Block{
		header:       &cpy,
		transactions: b.transactions,
		uncles:       b.uncles,
		producers:b.producers,
		Voters:b.Voters,
	}
}
func (b *Block) WithBody(transactions []*Transaction, uncles []*Header, producers Producers, voters Voters) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
		uncles:       make([]*Header, len(uncles)),
		producers:nil,
		Voters:nil,
	}
	copy(block.transactions, transactions)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	for _, producer := range producers {
		p := Producer{
			Addr:common.HexToAddress(producer.Addr.Hex()),
		}
		p.Vote = producer.SafeVote()
		block.producers = append(block.producers, p)
	}
	for _, voter := range voters {
		p := Voter{
			Addr:common.HexToAddress(voter.Addr.Hex()),
		}
		p.Vote = new(big.Int).Set(voter.Vote)
		p.Rank = voter.Rank
		block.Voters = append(block.Voters, p)
	}
	return block
}
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}
func (b *Block) String() string {
	str := fmt.Sprintf(`Block(#%v): Size: %v {
MinerHash: %x
%v
Transactions:
%v
Uncles:
%v
%v
%v
}
`, b.Number(), b.Size(), b.header.HashNoNonce(), b.header, b.transactions, b.uncles, b.producers, b.Voters)
	return str
}
func (h *Header) String() string {
	return fmt.Sprintf(`Header(%x):
[
	ParentHash:	    %x
	UncleHash:	    %x
	ProducerHash:	%x
	VoterHash:		%x
	Coinbase:	    %x
	Root:		    %x
	TxSha		    %x
	ReceiptSha:	    %x
	Bloom:		    %x
	Difficulty:	    %v
	Number:		    %v
	GasLimit:	    %v
	GasUsed:	    %v
	Time:		    %v
	Extra:		    %s
	MixDigest:      %x
	Nonce:		    %x
]`, h.Hash(), h.ParentHash, h.ProducerHash, h.VoterHash, h.UncleHash, h.Coinbase, h.Root, h.TxHash, h.ReceiptHash, h.Bloom, h.Difficulty, h.Number, h.GasLimit, h.GasUsed, h.Time, h.Extra, h.MixDigest, h.Nonce)
}
type Blocks []*Block
type BlockBy func(b1, b2 *Block) bool
func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}
type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}
func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }
func Number(b1, b2 *Block) bool { return b1.header.Number.Cmp(b2.header.Number) < 0 }
