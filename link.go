package lib

import (
	"fmt"
	"io"
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

	Silence bool
}

func NewLink(reader io.ReadCloser, writer io.WriteCloser, sendBufferSize int, protoFactory ServerProtocolFactory, recv chan<- LinkMessage) *Link {
	l := &Link{
		name:      "unnamed",
		readChan:  reader,
		writeChan: writer,
		recv:      recv,
		trans:     make(chan LinkMessage),
		exit:      make(chan bool, 1),
	}
	l.sq = NewSendQ(writer, sendBufferSize)
	l.reader = protoFactory.Reader(reader)
	l.writer = protoFactory.Writer(l.sq)
	go l.readLoop()
	go l.controlLoop()
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
	close(l.exit)
	l.sq.Close()
}

func (l *Link) Shutdown() {
	close(l.exit)
	l.sq.FlushAndClose()
}

func (l *Link) controlLoop() {
	for {
		select {
		case cleanExit := <-l.exit:
			if cleanExit {
				l.sq.FlushAndClose()
			} else {
				l.sq.Close()
			}
			for _ = range l.trans {
			}
			return
		case msg := <-l.trans:
			l.recv <- msg
			break
		case sqErr := <-l.sq.ErrChan():
			if sqErr != nil {
				l.recv <- LinkMessage{l, nil, sqErr}
			}
			break
		}
	}
}

func (l *Link) readLoop() {
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
