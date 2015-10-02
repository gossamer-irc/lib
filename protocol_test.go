package lib

import (
	"io"
	"testing"
)

func setupProtocolReaderWriter() (ServerProtocolReader, ServerProtocolWriter, io.Closer) {
	r, w := io.Pipe()
	return NewGobServerProtocolReader(r), NewGobServerProtocolWriter(w), w
}

func TestEncodeDecodeHello(t *testing.T) {
	r, w, _ := setupProtocolReaderWriter()
	hello := SSHello{1, 123, "server.name", "server description", "test"}
	go func() {
		err := w.WriteMessage(hello)
		if err != nil {
			t.Fatal(err)
		}
	}()
	read, err := r.ReadMessage()
	readHello, ok := read.(*SSHello)
	if !ok {
		t.Fatal("Didn't get an SSHello back")
	}
	if err != nil {
		t.Fatal(err)
	}
	if readHello.Name != "server.name" {
		t.Errorf("Expected 'server.name', got '%s'", readHello.Name)
	}
}

func TestReadMessageWhenClosed(t *testing.T) {
	r, _, c := setupProtocolReaderWriter()
	c.Close()
	_, err := r.ReadMessage()
	if err != io.EOF {
		t.Errorf("Expected EOF, got '%s'", err)
	}
}
