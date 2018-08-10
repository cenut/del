package whisperv2
import (
	"testing"
	"time"
	"github.com/DEL-ORG/del/p2p"
	"github.com/DEL-ORG/del/p2p/discover"
)
type testPeer struct {
	client *Whisper
	stream *p2p.MsgPipeRW
	termed chan struct{}
}
func startTestPeer() *testPeer {
	remote := p2p.NewPeer(discover.NodeID{}, "", nil)
	tester, tested := p2p.MsgPipe()
	client := New()
	client.Start(nil)
	termed := make(chan struct{})
	go func() {
		defer client.Stop()
		defer close(termed)
		defer tested.Close()
		client.handlePeer(remote, tested)
	}()
	return &testPeer{
		client: client,
		stream: tester,
		termed: termed,
	}
}
func startTestPeerInited() (*testPeer, error) {
	peer := startTestPeer()
	if err := p2p.ExpectMsg(peer.stream, statusCode, []uint64{protocolVersion}); err != nil {
		peer.stream.Close()
		return nil, err
	}
	if err := p2p.SendItems(peer.stream, statusCode, protocolVersion); err != nil {
		peer.stream.Close()
		return nil, err
	}
	return peer, nil
}
func TestPeerStatusMessage(t *testing.T) {
	tester := startTestPeer()
	if err := p2p.ExpectMsg(tester.stream, statusCode, []uint64{protocolVersion}); err != nil {
		t.Fatalf("status message mismatch: %v", err)
	}
	tester.stream.Close()
	select {
	case <-tester.termed:
	case <-time.After(time.Second):
		t.Fatalf("local close timed out")
	}
}
func TestPeerHandshakeFail(t *testing.T) {
	tester := startTestPeer()
	if err := p2p.ExpectMsg(tester.stream, statusCode, []uint64{protocolVersion}); err != nil {
		t.Fatalf("status message mismatch: %v", err)
	}
	if err := p2p.SendItems(tester.stream, messagesCode); err != nil {
		t.Fatalf("failed to send malformed status: %v", err)
	}
	select {
	case <-tester.termed:
	case <-time.After(time.Second):
		t.Fatalf("remote close timed out")
	}
}
func TestPeerHandshakeSuccess(t *testing.T) {
	tester := startTestPeer()
	if err := p2p.ExpectMsg(tester.stream, statusCode, []uint64{protocolVersion}); err != nil {
		t.Fatalf("status message mismatch: %v", err)
	}
	if err := p2p.SendItems(tester.stream, statusCode, protocolVersion); err != nil {
		t.Fatalf("failed to send status: %v", err)
	}
	select {
	case <-tester.termed:
		t.Fatalf("valid handshake disconnected")
	case <-time.After(100 * time.Millisecond):
	}
	tester.stream.Close()
	select {
	case <-tester.termed:
	case <-time.After(time.Second):
		t.Fatalf("local close timed out")
	}
}
func TestPeerSend(t *testing.T) {
	tester, err := startTestPeerInited()
	if err != nil {
		t.Fatalf("failed to start initialized peer: %v", err)
	}
	defer tester.stream.Close()
	message := NewMessage([]byte("peer broadcast test message"))
	envelope, err := message.Wrap(DefaultPoW, Options{
		TTL: DefaultTTL,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := tester.client.Send(envelope); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	payload := []interface{}{envelope}
	if err := p2p.ExpectMsg(tester.stream, messagesCode, payload); err != nil {
		t.Fatalf("message mismatch: %v", err)
	}
	if err := tester.client.Send(envelope); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	if err := p2p.ExpectMsg(tester.stream, messagesCode, []interface{}{}); err != nil {
		t.Fatalf("message mismatch: %v", err)
	}
}
func TestPeerDeliver(t *testing.T) {
	tester, err := startTestPeerInited()
	if err != nil {
		t.Fatalf("failed to start initialized peer: %v", err)
	}
	defer tester.stream.Close()
	arrived := make(chan struct{}, 1)
	tester.client.Watch(Filter{
		Fn: func(message *Message) {
			arrived <- struct{}{}
		},
	})
	message := NewMessage([]byte("peer broadcast test message"))
	envelope, err := message.Wrap(DefaultPoW, Options{
		TTL: DefaultTTL,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := p2p.Send(tester.stream, messagesCode, []*Envelope{envelope}); err != nil {
		t.Fatalf("failed to transfer message: %v", err)
	}
	select {
	case <-arrived:
	case <-time.After(time.Second):
		t.Fatalf("message delivery timeout")
	}
	if err := p2p.Send(tester.stream, messagesCode, []*Envelope{envelope}); err != nil {
		t.Fatalf("failed to transfer message: %v", err)
	}
	select {
	case <-time.After(2 * transmissionCycle):
	case <-arrived:
		t.Fatalf("repeating message arrived")
	}
}
func TestPeerMessageExpiration(t *testing.T) {
	tester, err := startTestPeerInited()
	if err != nil {
		t.Fatalf("failed to start initialized peer: %v", err)
	}
	defer tester.stream.Close()
	tester.client.peerMu.RLock()
	if peers := len(tester.client.peers); peers != 1 {
		t.Fatalf("peer pool size mismatch: have %v, want %v", peers, 1)
	}
	var peer *peer
	for peer = range tester.client.peers {
		break
	}
	tester.client.peerMu.RUnlock()
	message := NewMessage([]byte("peer test message"))
	envelope, err := message.Wrap(DefaultPoW, Options{
		TTL: time.Second,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := tester.client.Send(envelope); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	payload := []interface{}{envelope}
	if err := p2p.ExpectMsg(tester.stream, messagesCode, payload); err != nil {
		if err := p2p.ExpectMsg(tester.stream, messagesCode, payload); err != nil {
			t.Fatalf("message mismatch: %v", err)
		}
	}
	if !peer.known.Has(envelope.Hash()) {
		t.Fatalf("message not found in cache")
	}
	exp := time.Now().Add(time.Second + 2*expirationCycle + 100*time.Millisecond)
	for time.Now().Before(exp) {
		if err := p2p.ExpectMsg(tester.stream, messagesCode, []interface{}{}); err != nil {
			t.Fatalf("message mismatch: %v", err)
		}
	}
	if peer.known.Has(envelope.Hash()) {
		t.Fatalf("message not expired from cache")
	}
}
