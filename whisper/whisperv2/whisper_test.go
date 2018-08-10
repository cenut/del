package whisperv2
import (
	"testing"
	"time"
	"github.com/DEL-ORG/del/p2p"
	"github.com/DEL-ORG/del/p2p/discover"
)
func startTestCluster(n int) []*Whisper {
	nodes := make([]*p2p.Peer, n)
	for i := 0; i < n; i++ {
		nodes[i] = p2p.NewPeer(discover.NodeID{}, "", nil)
	}
	whispers := make([]*Whisper, n)
	for i := 0; i < n; i++ {
		whispers[i] = New()
		whispers[i].Start(nil)
	}
	for i := 1; i < n; i++ {
		src, dst := p2p.MsgPipe()
		go whispers[0].handlePeer(nodes[i], src)
		go whispers[i].handlePeer(nodes[0], dst)
	}
	return whispers
}
func TestSelfMessage(t *testing.T) {
	client := startTestCluster(1)[0]
	self := client.NewIdentity()
	done := make(chan struct{})
	client.Watch(Filter{
		To: &self.PublicKey,
		Fn: func(msg *Message) {
			close(done)
		},
	})
	msg := NewMessage([]byte("self whisper"))
	envelope, err := msg.Wrap(DefaultPoW, Options{
		From: self,
		To:   &self.PublicKey,
		TTL:  DefaultTTL,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := client.Send(envelope); err != nil {
		t.Fatalf("failed to send self-message: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("self-message receive timeout")
	}
}
func TestDirectMessage(t *testing.T) {
	cluster := startTestCluster(2)
	sender := cluster[0]
	senderId := sender.NewIdentity()
	recipient := cluster[1]
	recipientId := recipient.NewIdentity()
	done := make(chan struct{})
	recipient.Watch(Filter{
		To: &recipientId.PublicKey,
		Fn: func(msg *Message) {
			close(done)
		},
	})
	msg := NewMessage([]byte("direct whisper"))
	envelope, err := msg.Wrap(DefaultPoW, Options{
		From: senderId,
		To:   &recipientId.PublicKey,
		TTL:  DefaultTTL,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := sender.Send(envelope); err != nil {
		t.Fatalf("failed to send direct message: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("direct message receive timeout")
	}
}
func TestAnonymousBroadcast(t *testing.T) {
	testBroadcast(true, t)
}
func TestIdentifiedBroadcast(t *testing.T) {
	testBroadcast(false, t)
}
func testBroadcast(anonymous bool, t *testing.T) {
	cluster := startTestCluster(3)
	sender := cluster[1]
	targets := cluster[1:]
	for _, target := range targets {
		if !anonymous {
			target.NewIdentity()
		}
	}
	dones := make([]chan struct{}, len(targets))
	for i := 0; i < len(targets); i++ {
		done := make(chan struct{}) 
		dones[i] = done
		targets[i].Watch(Filter{
			Topics: NewFilterTopicsFromStringsFlat("broadcast topic"),
			Fn: func(msg *Message) {
				close(done)
			},
		})
	}
	msg := NewMessage([]byte("broadcast whisper"))
	envelope, err := msg.Wrap(DefaultPoW, Options{
		Topics: NewTopicsFromStrings("broadcast topic"),
		TTL:    DefaultTTL,
	})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := sender.Send(envelope); err != nil {
		t.Fatalf("failed to send broadcast message: %v", err)
	}
	timeout := time.After(time.Second)
	for _, done := range dones {
		select {
		case <-done:
		case <-timeout:
			t.Fatalf("broadcast message receive timeout")
		}
	}
}
func TestMessageExpiration(t *testing.T) {
	node := startTestCluster(1)[0]
	message := NewMessage([]byte("expiring message"))
	envelope, err := message.Wrap(DefaultPoW, Options{TTL: time.Second})
	if err != nil {
		t.Fatalf("failed to wrap message: %v", err)
	}
	if err := node.Send(envelope); err != nil {
		t.Fatalf("failed to inject message: %v", err)
	}
	node.poolMu.RLock()
	_, found := node.messages[envelope.Hash()]
	node.poolMu.RUnlock()
	if !found {
		t.Fatalf("message not found in cache")
	}
	time.Sleep(time.Second)         
	time.Sleep(2 * expirationCycle) 
	node.poolMu.RLock()
	_, found = node.messages[envelope.Hash()]
	node.poolMu.RUnlock()
	if found {
		t.Fatalf("message not expired from cache")
	}
	node.add(envelope)
	node.poolMu.RLock()
	_, found = node.messages[envelope.Hash()]
	node.poolMu.RUnlock()
	if found {
		t.Fatalf("message was added to cache")
	}
}
