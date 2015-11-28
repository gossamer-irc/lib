package lib

import (
	"io"
	"sync"
	"testing"
)

// Verify that nothing explodes when a link is created & destroyed.
func TestLinkStartupShutdown(t *testing.T) {
	wg := &sync.WaitGroup{}
	r, w := io.Pipe()
	recv := make(chan LinkMessage)
	l := NewLink(r, w, 1024, GobServerProtocolFactory, recv, wg)
	l.Close()
	wg.Wait()
}

// Verify that a link in loopback configuration can receive a single message.
func TestLinkSingleMessage(t *testing.T) {
	wg := &sync.WaitGroup{}
	r, w := io.Pipe()
	recv := make(chan LinkMessage, 1)
	l := NewLink(r, w, 1024, GobServerProtocolFactory, recv, wg)
	hello := SSHello{1, 123, "server.name", "server description", "test"}
	l.WriteMessage(hello)
	msg := <-recv
	recvHello, ok := msg.msg.(*SSHello)
	if !ok {
		t.Fatalf("Expected SSHello message.")
	}
	if recvHello.Name != "server.name" {
		t.Errorf("Expected 'server.name', got '%s'", recvHello.Name)
	}
	l.Close()
	wg.Wait()
}

// Verify that links buffer and deliver all messages sent just before a Shutdown().
func TestLinkShutdown(t *testing.T) {
	wg := &sync.WaitGroup{}
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	recv1 := make(chan LinkMessage, 2)
	recv2 := make(chan LinkMessage, 2)
	l1 := NewLink(r1, w2, 1024, GobServerProtocolFactory, recv1, wg)
	l2 := NewLink(r2, w1, 1024, GobServerProtocolFactory, recv2, wg)

	hello := SSHello{1, 123, "server.name", "server description", "test"}

	// Send message 4 times.
	l1.WriteMessage(hello)
	l1.WriteMessage(hello)
	l1.WriteMessage(hello)
	l1.WriteMessage(hello)

	// Now, shutdown the link. The sends should still go through.
	l1.Shutdown()

	// Expect to receive 4 hellos.
	for i := 1; i <= 4; i++ {
		r := <-recv2
		msg, ok := r.msg.(*SSHello)
		if !ok {
			t.Fatalf("Expected SSHello message.")
		}
		if msg.Name != "server.name" {
			t.Errorf("Expected 'server.name', got '%s'", msg.Name)
		}
	}

	// Expect to see an EOF.
	r := <-recv2
	if r.err != io.EOF {
		t.Errorf("Expected EOF, got '%s'", r.err)
	}

	l2.Close()
	defer wg.Wait()
}

// Verify that two links can pass messages back and forth.
func TestLinkToLink(t *testing.T) {
	wg := &sync.WaitGroup{}
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	recv1 := make(chan LinkMessage, 2)
	recv2 := make(chan LinkMessage, 2)
	l1 := NewLink(r1, w2, 1024, GobServerProtocolFactory, recv1, wg)
	l2 := NewLink(r2, w1, 1024, GobServerProtocolFactory, recv2, wg)

	hello1 := SSHello{1, 123, "server.a", "server description", "test"}
	hello2 := SSHello{1, 123, "server.b", "server description", "test"}

	l1.WriteMessage(hello1)
	l2.WriteMessage(hello2)

	msg1, ok := (<-recv1).msg.(*SSHello)
	if !ok {
		t.Fatalf("Expected SSHello message from #2.")
	}
	msg2, ok := (<-recv2).msg.(*SSHello)
	if !ok {
		t.Fatalf("Expected SSHello message from #1.")
	}
	if msg1.Name != "server.b" {
		t.Errorf("Expected 'server.b', got '%s'", msg1.Name)
	}
	if msg2.Name != "server.a" {
		t.Errorf("Expected 'server.a', got '%s'", msg2.Name)
	}
	l1.Close()
	l2.Close()
	wg.Wait()
}
