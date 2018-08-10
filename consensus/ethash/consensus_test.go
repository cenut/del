package ethash
import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"github.com/DEL-ORG/del/common/math"
	"github.com/DEL-ORG/del/core/types"
	"github.com/DEL-ORG/del/params"
)
type diffTest struct {
	ParentTimestamp    uint64
	ParentDifficulty   *big.Int
	CurrentTimestamp   uint64
	CurrentBlocknumber *big.Int
	CurrentDifficulty  *big.Int
}
func (d *diffTest) UnmarshalJSON(b []byte) (err error) {
	var ext struct {
		ParentTimestamp    string
		ParentDifficulty   string
		CurrentTimestamp   string
		CurrentBlocknumber string
		CurrentDifficulty  string
	}
	if err := json.Unmarshal(b, &ext); err != nil {
		return err
	}
	d.ParentTimestamp = math.MustParseUint64(ext.ParentTimestamp)
	d.ParentDifficulty = math.MustParseBig256(ext.ParentDifficulty)
	d.CurrentTimestamp = math.MustParseUint64(ext.CurrentTimestamp)
	d.CurrentBlocknumber = math.MustParseBig256(ext.CurrentBlocknumber)
	d.CurrentDifficulty = math.MustParseBig256(ext.CurrentDifficulty)
	return nil
}
func TestCalcDifficulty(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "tests", "testdata", "BasicTests", "difficulty.json"))
	if err != nil {
		t.Skip(err)
	}
	defer file.Close()
	tests := make(map[string]diffTest)
	err = json.NewDecoder(file).Decode(&tests)
	if err != nil {
		t.Fatal(err)
	}
	config := &params.ChainConfig{HomesteadBlock: big.NewInt(1150000)}
	for name, test := range tests {
		number := new(big.Int).Sub(test.CurrentBlocknumber, big.NewInt(1))
		diff := CalcDifficulty(config, test.CurrentTimestamp, &types.Header{
			Number:     number,
			Time:       new(big.Int).SetUint64(test.ParentTimestamp),
			Difficulty: test.ParentDifficulty,
		})
		if diff.Cmp(test.CurrentDifficulty) != 0 {
			t.Error(name, "failed. Expected", test.CurrentDifficulty, "and calculated", diff)
		}
	}
}
func TestEthash_CalProducers(t *testing.T) {
	for len := 1; len < 500; len++ {
		for limit := len; limit < len * 10; limit++ {
			jmp := (limit - len) / len
			remain := limit - len*jmp - len
			if remain < 0 {
				t.Error("remain error")
			}
			total := 0
			count := 0
			for i := 0; i < limit && count < len; {
				total++
				count++
				for j := 0; j < jmp; j++ {
					total++
				}
				i+=jmp + 1
				if remain > 0 {
					i++
					remain--
					total++
				}
			}
			if total != limit {
				t.Error("CalProducers error")
			}
		}
	}
}
