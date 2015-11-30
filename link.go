package lib

import (
	"fmt"
	"io"
	"sync"
)

// Represents a link between a local and a directly connected remote node.
type Link struct {
	name                string
	readChan, writeChan io.Closer
	reader              ServerProtocolReader
	writer              ServerProtocolWriter
	sq                  *SendQ

	recv  chan<- LinkMessage
	trans chan LinkMessage
	exit  chan bool

	// Ensures the link can only be closed once.
	closed bool

	Silence bool
}

func NewLink(reader io.ReadCloser, writer io.WriteCloser, sendBufferSize int, protoFactory ServerProtocolFactory, recv chan<- LinkMessage, wg *sync.WaitGroup) *Link {
	l := &Link{
		name:      "unnamed",
		readChan:  reader,
		writeChan: writer,
		recv:      recv,
		trans:     make(chan LinkMessage, 1),
		exit:      make(chan bool, 1),
	}
	l.sq = NewSendQ(writer, sendBufferSize, wg)
	l.reader = protoFactory.Reader(reader)
	l.writer = protoFactory.Writer(l.sq)
	wg.Add(2)
	go l.readLoop(wg)
	go l.controlLoop(wg)
	return l
}

func (l *Link) SetName(name string) {
	l.name = name
	l.sq.name = fmt.Sprintf("sq:%s", name)
}

func (l *Link) WriteMessage(msg SSMessage) error {
	return l.writer.WriteMessage(msg)
}

func (l *Link) Close() {
	l.readChan.Close()
	if !l.closed {
		l.closed = true
		close(l.exit)
	}
	l.sq.Close()
}

func (l *Link) Shutdown() {
	l.readChan.Close()
	if !l.closed {
		l.closed = true
		close(l.exit)
	}
	l.sq.FlushAndClose()
}

func (l *Link) controlLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-l.exit:
			trans := l.trans
			if trans != nil {
				for _ = range trans {
				}
			}
			close(l.recv)
			return
		case msg, ok := <-l.trans:
			if !ok {
				l.trans = nil
			} else {
				l.recv <- msg
			}
		case sqErr := <-l.sq.ErrChan():
			if sqErr != nil {
				l.recv <- LinkMessage{l, nil, sqErr}
			}
		}
	}
}

func (l *Link) readLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		// Look for an exit signal. Nothing will actually be received,
		// but the channel can be closed.
		select {
		case <-l.exit:
			close(l.trans)
			return
		default:
			// Attempt to read.
			msg, err := l.reader.ReadMessage()
			l.trans <- LinkMessage{l, msg, err}
			// Don't attempt to read anymore.
			if err != nil {
				close(l.trans)
				return
			}
		}
	}
}

// A message sent pertaining to a particular link.
type LinkMessage struct {
	link *Link
	msg  SSMessage
	err  error
}
