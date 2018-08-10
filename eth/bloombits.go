package eth
import (
	"time"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/bitutil"
	"github.com/DEL-ORG/del/core"
	"github.com/DEL-ORG/del/core/bloombits"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/ethdb"
	"github.com/DEL-ORG/del/params"
)
const (
	bloomServiceThreads = 16
	bloomFilterThreads = 3
	bloomRetrievalBatch = 16
	bloomRetrievalWait = time.Duration(0)
)
func (eth *Ethereum) startBloomHandlers() {
	for i := 0; i < bloomServiceThreads; i++ {
		go func() {
			for {
				select {
				case <-eth.shutdownChan:
					return
				case request := <-eth.bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
						head := core.GetCanonicalHash(eth.chainDb, (section+1)*params.BloomBitsBlocks-1)
						if compVector, err := core.GetBloomBits(eth.chainDb, task.Bit, section, head); err == nil {
							if blob, err := bitutil.DecompressBytes(compVector, int(params.BloomBitsBlocks)/8); err == nil {
								task.Bitsets[i] = blob
							} else {
								task.Error = err
							}
						} else {
							task.Error = err
						}
					}
					request <- task
				}
			}
		}()
	}
}
const (
	bloomConfirms = 256
	bloomThrottling = 100 * time.Millisecond
)
type BloomIndexer struct {
	size uint64 
	db  ethdb.Database       
	gen *bloombits.Generator 
	section uint64      
	head    common.Hash 
}
func NewBloomIndexer(db ethdb.Database, size uint64) *core.ChainIndexer {
	backend := &BloomIndexer{
		db:   db,
		size: size,
	}
	table := ethdb.NewTable(db, string(core.BloomBitsIndexPrefix))
	return core.NewChainIndexer(db, table, backend, size, bloomConfirms, bloomThrottling, "bloombits")
}
func (b *BloomIndexer) Reset(section uint64, lastSectionHead common.Hash) error {
	gen, err := bloombits.NewGenerator(uint(b.size))
	b.gen, b.section, b.head = gen, section, common.Hash{}
	return err
}
func (b *BloomIndexer) Process(header *types.Header) {
	b.gen.AddBloom(uint(header.Number.Uint64()-b.section*b.size), header.Bloom)
	b.head = header.Hash()
}
func (b *BloomIndexer) Commit() error {
	batch := b.db.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := b.gen.Bitset(uint(i))
		if err != nil {
			return err
		}
		core.WriteBloomBits(batch, uint(i), b.section, b.head, bitutil.CompressBytes(bits))
	}
	return batch.Write()
}
