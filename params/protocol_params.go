package params
import (
	"math/big"
	"math"
	"errors"
)
var (
	TargetGasLimit uint64 = GenesisGasLimit 
)
const (
	GasLimitBoundDivisor uint64 = 1024    
	MinGasLimit          uint64 = 453718400    
	GenesisGasLimit      uint64 = 4712388 
	MaximumExtraDataSize  uint64 = 32    
	ExpByteGas            uint64 = 10    
	SloadGas              uint64 = 50    
	CallValueTransferGas  uint64 = 9000  
	CallNewAccountGas     uint64 = 25000 
	TxGas                 uint64 = 21000 
	TxGasContractCreation uint64 = 53000 
	TxDataZeroGas         uint64 = 4     
	QuadCoeffDiv          uint64 = 512   
	SstoreSetGas          uint64 = 20000 
	LogDataGas            uint64 = 8     
	CallStipend           uint64 = 2300  
	Sha3Gas          uint64 = 30    
	Sha3WordGas      uint64 = 6     
	SstoreResetGas   uint64 = 5000  
	SstoreClearGas   uint64 = 5000  
	SstoreRefundGas  uint64 = 15000 
	JumpdestGas      uint64 = 1     
	EpochDuration    uint64 = 30000 
	CallGas          uint64 = 40    
	CreateDataGas    uint64 = 200   
	CallCreateDepth  uint64 = 1024  
	ExpGas           uint64 = 10    
	LogGas           uint64 = 375   
	CopyGas          uint64 = 3     
	StackLimit       uint64 = 1024  
	TierStepGas      uint64 = 0     
	LogTopicGas      uint64 = 375   
	CreateGas        uint64 = 32000 
	SuicideRefundGas uint64 = 24000 
	MemoryGas        uint64 = 3     
	TxDataNonZeroGas uint64 = 68    
	MaxCodeSize = 24576 
	EcrecoverGas            uint64 = 3000   
	Sha256BaseGas           uint64 = 60     
	Sha256PerWordGas        uint64 = 12     
	Ripemd160BaseGas        uint64 = 600    
	Ripemd160PerWordGas     uint64 = 120    
	IdentityBaseGas         uint64 = 15     
	IdentityPerWordGas      uint64 = 3      
	ModExpQuadCoeffDiv      uint64 = 20     
	Bn256AddGas             uint64 = 500    
	Bn256ScalarMulGas       uint64 = 40000  
	Bn256PairingBaseGas     uint64 = 100000 
	Bn256PairingPerPointGas uint64 = 80000  
)
var (
	DifficultyBoundDivisor = big.NewInt(1024)   
	GenesisDifficulty      = big.NewInt(3338160) 
	MinimumDifficulty      = big.NewInt(3338160) 
	DurationLimit          = big.NewInt(13)     
)
func IntrinsicGas(data []byte) (uint64, error) {
	var gas uint64
	
	gas = TxGas
	if len(data) > 0 {
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		if (math.MaxUint64-gas)/TxDataNonZeroGas < nz {
			return 0, errors.New("out of gas")
		}
		gas += nz * TxDataNonZeroGas
		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/TxDataZeroGas < z {
			return 0, errors.New("out of gas")
		}
		gas += z * TxDataZeroGas
	}
	return gas, nil
}
