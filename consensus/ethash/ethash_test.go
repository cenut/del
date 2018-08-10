package ethash
import (
	"testing"
	"github.com/DEL-ORG/del/core/types"
	"math/big"
	"io/ioutil"
	"os"
	"sync"
	"math/rand"
)


func TestTestMode(t *testing.T) {
	head := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(100)}
	ethash := NewTester()
	block, err := ethash.Seal(nil, types.NewBlockWithHeader(head), nil)
	if err != nil {
		t.Fatalf("failed to seal block: %v", err)
	}
	head.Nonce = types.EncodeNonce(block.Nonce())
	head.MixDigest = block.MixDigest()
	if err := ethash.VerifySeal(nil, head); err != nil {
		t.Fatalf("unexpected verification error: %v", err)
	}
}
func TestCacheFileEvict(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "ethash-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	e := New(Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: tmpdir, PowMode: ModeTest})
	workers := 8
	epochs := 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go verifyTest(&wg, e, i, epochs)
	}
	wg.Wait()
}
func verifyTest(wg *sync.WaitGroup, e *Ethash, workerIndex, epochs int) {
	defer wg.Done()
	const wiggle = 4 * epochLength
	r := rand.New(rand.NewSource(int64(workerIndex)))
	for epoch := 0; epoch < epochs; epoch++ {
		block := int64(epoch)*epochLength - wiggle/2 + r.Int63n(wiggle)
		if block < 0 {
			block = 0
		}
		head := &types.Header{Number: big.NewInt(block), Difficulty: big.NewInt(100)}
		e.VerifySeal(nil, head)
	}
}
