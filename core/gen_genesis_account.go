package core
import (
	"encoding/json"
	"errors"
	"math/big"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/common/hexutil"
	"github.com/DEL-ORG/del/common/math"
)
var _ = (*genesisAccountMarshaling)(nil)
func (g GenesisAccount) MarshalJSON() ([]byte, error) {
	type GenesisAccount struct {
		Code       hexutil.Bytes               `json:"code,omitempty"`
		Storage    map[storageJSON]storageJSON `json:"storage,omitempty"`
		Balance    *math.HexOrDecimal256       `json:"balance" gencodec:"required"`
		Freeze    *math.HexOrDecimal256       `json:"freeze" gencodec:"required"`
		Addr common.Address               `json:"addr" gencodec:"required"`
		Producer bool              `json:"producer" gencodec:"required"`
		Nonce      math.HexOrDecimal64         `json:"nonce,omitempty"`
		PrivateKey hexutil.Bytes               `json:"secretKey,omitempty"`
	}
	var enc GenesisAccount
	enc.Code = g.Code
	if g.Storage != nil {
		enc.Storage = make(map[storageJSON]storageJSON, len(g.Storage))
		for k, v := range g.Storage {
			enc.Storage[storageJSON(k)] = storageJSON(v)
		}
	}
	enc.Balance = (*math.HexOrDecimal256)(g.Balance)
	enc.Freeze = (*math.HexOrDecimal256)(g.Freeze)
	enc.Addr = g.Addr
	enc.Producer = g.Producer
	enc.Nonce = math.HexOrDecimal64(g.Nonce)
	enc.PrivateKey = g.PrivateKey
	return json.Marshal(&enc)
}
func (g *GenesisAccount) UnmarshalJSON(input []byte) error {
	type GenesisAccount struct {
		Code       *hexutil.Bytes              `json:"code,omitempty"`
		Storage    map[storageJSON]storageJSON `json:"storage,omitempty"`
		Balance    *math.HexOrDecimal256       `json:"balance" gencodec:"required"`
		Freeze    *math.HexOrDecimal256       `json:"freeze" gencodec:"required"`
		Addr *hexutil.Bytes              `json:"addr" gencodec:"required"`
		Producer bool             `json:"producer" gencodec:"required"`
		Nonce      *math.HexOrDecimal64        `json:"nonce,omitempty"`
		PrivateKey *hexutil.Bytes              `json:"secretKey,omitempty"`
	}
	var dec GenesisAccount
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Code != nil {
		g.Code = *dec.Code
	}
	if dec.Storage != nil {
		g.Storage = make(map[common.Hash]common.Hash, len(dec.Storage))
		for k, v := range dec.Storage {
			g.Storage[common.Hash(k)] = common.Hash(v)
		}
	}
	if dec.Balance == nil {
		return errors.New("missing required field 'balance' for GenesisAccount")
	}
	g.Balance = (*big.Int)(dec.Balance)
	g.Freeze = (*big.Int)(dec.Freeze)
	g.Producer = dec.Producer
	if dec.Addr != nil {
		g.Addr = common.BytesToAddress(*dec.Addr)
	}
	if dec.Nonce != nil {
		g.Nonce = uint64(*dec.Nonce)
	}
	if dec.PrivateKey != nil {
		g.PrivateKey = *dec.PrivateKey
	}
	return nil
}
