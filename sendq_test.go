package lib

import (
	"bufio"
	"errors"
	"io"
	"testing"
)

// Tests that a SendQ can be initialized and shutdown properly.
func TestSendQStartupShutdown(t *testing.T) {
	r, w := io.Pipe()
	sq := NewSendQ(w, 100)
	sq.Close()
	b := make([]byte, 10)
	n, err := r.Read(b)
	if n != 0 || err != io.EOF {
		t.Fatalf("Expected EOF, got %s", err)
	}
}

// Tests that a SendQ buffers sent values and sends them through properly.
func TestSendQBasicSend(t *testing.T) {
	r, w := io.Pipe()
	bufR := bufio.NewReader(r)

	sq := NewSendQ(w, 100)
	sq.Write([]byte("hello "))
	sq.Write([]byte("world"))
	sq.FlushAndClose()
	data, err := bufR.ReadString('\n')
	if data != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", data)
	}
	if err != io.EOF {
		t.Errorf("Expected EOF, got %s", err)
	}
}

// Tests that a SendQ reports overflow errors properly.
func TestSendQBufferOverflow(t *testing.T) {
	r, w := io.Pipe()
	bufR := bufio.NewReader(r)

	sq := NewSendQ(w, 10)
	sq.Write([]byte("hello "))
	sq.Write([]byte("world"))
	_, err := bufR.ReadString('\n')
	if err != io.EOF {
		t.Errorf("Expected EOF, got %s", err)
	}
	err = <-sq.ErrChan()
	if err != SendQFull {
		t.Errorf("Expected SendQFull, got %s", err)
	}
	if unknown, done := <-sq.ErrChan(); done {
		t.Errorf("Expected error channel to close, but it didn't. Instead, got '%s'", unknown)
	}
}

// Tests that a SendQ passes errors through with proper context.
func TestSendQErrorPropagation(t *testing.T) {
	r, w := io.Pipe()
	sq := NewSendQ(w, 100)
	expected := errors.New("test")
	r.CloseWithError(expected)
	sq.Write([]byte("this should fail"))
	err := <-sq.ErrChan()
	if err != expected {
		t.Errorf("Expected a known error, got %d", err)
	}
	if unknown, done := <-sq.ErrChan(); done {
		t.Errorf("Expected error channel to close, but it didn't. Instead, got '%s'", unknown)
	}
}
