package ethash
import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unsafe"
	"github.com/edsrzf/mmap-go"
	"github.com/DEL-ORG/del/consensus"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/rpc"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/rcrowley/go-metrics"
	"github.com/DEL-ORG/del/accounts"
)
var ErrInvalidDumpMagic = errors.New("invalid dump magic")
var (
	maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
	sharedEthash = New(Config{"", 3, 0, "", 1, 0, ModeNormal})
	algorithmRevision = 23
	dumpMagic = []uint32{0xbaddcafe, 0xfee1dead}
)
func isLittleEndian() bool {
	n := uint32(0x01020304)
	return *(*byte)(unsafe.Pointer(&n)) == 0x04
}
func memoryMap(path string) (*os.File, mmap.MMap, []uint32, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, nil, err
	}
	mem, buffer, err := memoryMapFile(file, false)
	if err != nil {
		file.Close()
		return nil, nil, nil, err
	}
	for i, magic := range dumpMagic {
		if buffer[i] != magic {
			mem.Unmap()
			file.Close()
			return nil, nil, nil, ErrInvalidDumpMagic
		}
	}
	return file, mem, buffer[len(dumpMagic):], err
}
func memoryMapFile(file *os.File, write bool) (mmap.MMap, []uint32, error) {
	flag := mmap.RDONLY
	if write {
		flag = mmap.RDWR
	}
	mem, err := mmap.Map(file, flag, 0)
	if err != nil {
		return nil, nil, err
	}
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&mem))
	header.Len /= 4
	header.Cap /= 4
	return mem, *(*[]uint32)(unsafe.Pointer(&header)), nil
}
func memoryMapAndGenerate(path string, size uint64, generator func(buffer []uint32)) (*os.File, mmap.MMap, []uint32, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, nil, err
	}
	temp := path + "." + strconv.Itoa(rand.Int())
	dump, err := os.Create(temp)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = dump.Truncate(int64(len(dumpMagic))*4 + int64(size)); err != nil {
		return nil, nil, nil, err
	}
	mem, buffer, err := memoryMapFile(dump, true)
	if err != nil {
		dump.Close()
		return nil, nil, nil, err
	}
	copy(buffer, dumpMagic)
	data := buffer[len(dumpMagic):]
	generator(data)
	if err := mem.Unmap(); err != nil {
		return nil, nil, nil, err
	}
	if err := dump.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := os.Rename(temp, path); err != nil {
		return nil, nil, nil, err
	}
	return memoryMap(path)
}
type lru struct {
	what string
	new  func(epoch uint64) interface{}
	mu   sync.Mutex
	cache      *simplelru.LRU
	future     uint64
	futureItem interface{}
}
func newlru(what string, maxItems int, new func(epoch uint64) interface{}) *lru {
	if maxItems <= 0 {
		maxItems = 1
	}
	cache, _ := simplelru.NewLRU(maxItems, func(key, value interface{}) {
		log.Trace("Evicted ethash "+what, "epoch", key)
	})
	return &lru{what: what, new: new, cache: cache}
}
func (lru *lru) get(epoch uint64) (item, future interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	item, ok := lru.cache.Get(epoch)
	if !ok {
		if lru.future > 0 && lru.future == epoch {
			item = lru.futureItem
		} else {
			log.Trace("Requiring new ethash "+lru.what, "epoch", epoch)
			item = lru.new(epoch)
		}
		lru.cache.Add(epoch, item)
	}
	if epoch < maxEpoch-1 && lru.future < epoch+1 {
		log.Trace("Requiring new future ethash "+lru.what, "epoch", epoch+1)
		future = lru.new(epoch + 1)
		lru.future = epoch + 1
		lru.futureItem = future
	}
	return item, future
}
type cache struct {
	epoch uint64    
	dump  *os.File  
	mmap  mmap.MMap 
	cache []uint32  
	once  sync.Once 
}
func newCache(epoch uint64) interface{} {
	return &cache{epoch: epoch}
}
func (c *cache) generate(dir string, limit int, test bool) {
	c.once.Do(func() {
		size := cacheSize(c.epoch*epochLength + 1)
		seed := seedHash(c.epoch*epochLength + 1)
		if test {
			size = 1024
		}
		if dir == "" {
			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
			return
		}
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", c.epoch)
		runtime.SetFinalizer(c, (*cache).finalizer)
		var err error
		c.dump, c.mmap, c.cache, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old ethash cache from disk")
			return
		}
		logger.Debug("Failed to load old ethash cache", "err", err)
		c.dump, c.mmap, c.cache, err = memoryMapAndGenerate(path, size, func(buffer []uint32) { generateCache(buffer, c.epoch, seed) })
		if err != nil {
			logger.Error("Failed to generate mapped ethash cache", "err", err)
			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
		}
		for ep := int(c.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}
func (c *cache) finalizer() {
	if c.mmap != nil {
		c.mmap.Unmap()
		c.dump.Close()
		c.mmap, c.dump = nil, nil
	}
}
type dataset struct {
	epoch   uint64    
	dump    *os.File  
	mmap    mmap.MMap 
	dataset []uint32  
	once    sync.Once 
}
func newDataset(epoch uint64) interface{} {
	return &dataset{epoch: epoch}
}
func (d *dataset) generate(dir string, limit int, test bool) {
	d.once.Do(func() {
		csize := cacheSize(d.epoch*epochLength + 1)
		dsize := datasetSize(d.epoch*epochLength + 1)
		seed := seedHash(d.epoch*epochLength + 1)
		if test {
			csize = 1024
			dsize = 32 * 1024
		}
		if dir == "" {
			cache := make([]uint32, csize/4)
			generateCache(cache, d.epoch, seed)
			d.dataset = make([]uint32, dsize/4)
			generateDataset(d.dataset, d.epoch, cache)
		}
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", d.epoch)
		runtime.SetFinalizer(d, (*dataset).finalizer)
		var err error
		d.dump, d.mmap, d.dataset, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old ethash dataset from disk")
			return
		}
		logger.Debug("Failed to load old ethash dataset", "err", err)
		cache := make([]uint32, csize/4)
		generateCache(cache, d.epoch, seed)
		d.dump, d.mmap, d.dataset, err = memoryMapAndGenerate(path, dsize, func(buffer []uint32) { generateDataset(buffer, d.epoch, cache) })
		if err != nil {
			logger.Error("Failed to generate mapped ethash dataset", "err", err)
			d.dataset = make([]uint32, dsize/2)
			generateDataset(d.dataset, d.epoch, cache)
		}
		for ep := int(d.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}
func (d *dataset) finalizer() {
	if d.mmap != nil {
		d.mmap.Unmap()
		d.dump.Close()
		d.mmap, d.dump = nil, nil
	}
}
func MakeCache(block uint64, dir string) {
	c := cache{epoch: block / epochLength}
	c.generate(dir, math.MaxInt32, false)
}
func MakeDataset(block uint64, dir string) {
	d := dataset{epoch: block / epochLength}
	d.generate(dir, math.MaxInt32, false)
}
type Mode uint
const (
	ModeNormal Mode = iota
	ModeShared
	ModeTest
	ModeFake
	ModeFullFake
)
type Config struct {
	CacheDir       string
	CachesInMem    int
	CachesOnDisk   int
	DatasetDir     string
	DatasetsInMem  int
	DatasetsOnDisk int
	PowMode        Mode
}
type SignerFn func(accounts.Account, []byte) ([]byte, error)
type Ethash struct {
	config Config
	caches   *lru 
	datasets *lru 
	rand     *rand.Rand    
	threads  int           
	update   chan struct{} 
	hashrate metrics.Meter 
	shared    *Ethash       
	fakeFail  uint64        
	fakeDelay time.Duration 
	
	lock sync.Mutex 
}
func New(config Config) *Ethash {
	if config.CachesInMem <= 0 {
		config.CachesInMem = 1
	}
	if config.CacheDir != "" && config.CachesOnDisk > 0 {
	}
	if config.DatasetDir != "" && config.DatasetsOnDisk > 0 {
	}
	return &Ethash{
		config:   config,
		caches:   newlru("cache", config.CachesInMem, newCache),
		datasets: newlru("dataset", config.DatasetsInMem, newDataset),
		update:   make(chan struct{}),
		hashrate: metrics.NewMeter(),
	}
}
func NewTester() *Ethash {
	return New(Config{CachesInMem: 1, PowMode: ModeTest})
}
func NewFaker() *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
	}
}
func NewFakeFailer(fail uint64) *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
		fakeFail: fail,
	}
}
func NewFakeDelayer(delay time.Duration) *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
		fakeDelay: delay,
	}
}
func NewFullFaker() *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFullFake,
		},
	}
}
func NewShared() *Ethash {
	return &Ethash{shared: sharedEthash}
}
func (ethash *Ethash) cache(block uint64) *cache {
	epoch := block / epochLength
	currentI, futureI := ethash.caches.get(epoch)
	current := currentI.(*cache)
	current.generate(ethash.config.CacheDir, ethash.config.CachesOnDisk, ethash.config.PowMode == ModeTest)
	if futureI != nil {
		future := futureI.(*cache)
		go future.generate(ethash.config.CacheDir, ethash.config.CachesOnDisk, ethash.config.PowMode == ModeTest)
	}
	return current
}
func (ethash *Ethash) dataset(block uint64) *dataset {
	epoch := block / epochLength
	currentI, futureI := ethash.datasets.get(epoch)
	current := currentI.(*dataset)
	current.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)
	if futureI != nil {
		future := futureI.(*dataset)
		go future.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)
	}
	return current
}
func (ethash *Ethash) Threads() int {
	ethash.lock.Lock()
	defer ethash.lock.Unlock()
	return ethash.threads
}
func (ethash *Ethash) SetThreads(threads int) {
	ethash.lock.Lock()
	defer ethash.lock.Unlock()
	if ethash.shared != nil {
		ethash.shared.SetThreads(threads)
		return
	}
	ethash.threads = threads
	select {
	case ethash.update <- struct{}{}:
	default:
	}
}
func (ethash *Ethash) Hashrate() float64 {
	return ethash.hashrate.Rate1()
}
func (ethash *Ethash) APIs(chain consensus.ChainReader) []rpc.API {
	return nil
}
func SeedHash(block uint64) []byte {
	return seedHash(block)
}
