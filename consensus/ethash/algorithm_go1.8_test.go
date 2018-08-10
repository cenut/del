// +build go1.8

package ethash
import "testing"
func TestSizeCalculations(t *testing.T) {
	for epoch, want := range cacheSizes {
		if size := calcCacheSize(epoch); size != want {
			t.Errorf("cache %d: cache size mismatch: have %d, want %d", epoch, size, want)
		}
	}
	for epoch, want := range datasetSizes {
		if size := calcDatasetSize(epoch); size != want {
			t.Errorf("dataset %d: dataset size mismatch: have %d, want %d", epoch, size, want)
		}
	}
}
