package downloader
import (
	"fmt"
	"github.com/DEL-ORG/del/core/types"
)
type peerDropFn func(id string)
type dataPack interface {
	PeerId() string
	Items() int
	Stats() string
}
type headerPack struct {
	peerId  string
	headers []*types.Header
}
func (p *headerPack) PeerId() string { return p.peerId }
func (p *headerPack) Items() int     { return len(p.headers) }
func (p *headerPack) Stats() string  { return fmt.Sprintf("%d", len(p.headers)) }
type bodyPack struct {
	peerId       string
	transactions [][]*types.Transaction
	uncles       [][]*types.Header
	producers []types.Producers
	voters []types.Voters
}
func (p *bodyPack) PeerId() string { return p.peerId }
func (p *bodyPack) Items() int {
	min := len(p.transactions)
	if len(p.uncles) < min {
		min = len(p.uncles)
	}
	if len(p.producers) < min {
		min = len(p.producers)
	}
	if len(p.voters) < min {
		min = len(p.voters)
	}
	return min
}
func (p *bodyPack) Stats() string { return fmt.Sprintf("%d:%d:%d:%d", len(p.transactions), len(p.uncles), len(p.producers), len(p.voters)) }
type receiptPack struct {
	peerId   string
	receipts [][]*types.Receipt
}
func (p *receiptPack) PeerId() string { return p.peerId }
func (p *receiptPack) Items() int     { return len(p.receipts) }
func (p *receiptPack) Stats() string  { return fmt.Sprintf("%d", len(p.receipts)) }
type statePack struct {
	peerId string
	states [][]byte
}
func (p *statePack) PeerId() string { return p.peerId }
func (p *statePack) Items() int     { return len(p.states) }
func (p *statePack) Stats() string  { return fmt.Sprintf("%d", len(p.states)) }
