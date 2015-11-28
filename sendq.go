package lib

import (
	"errors"
	"io"
	"sync"
)

var SendQFull error = errors.New("SendQFull")

// A SendQ is an io.Writer that implements fixed buffering. Once the buffer
// buffer is exhausted, a QueueFull error is raised. Writing to the buffer
// always succeeds, though - any errors are delivered asynchronously. All
// errors are fatal - the queue shuts down and becomes a noop after an error
// is encountered.
type SendQ struct {
	// Lock guarding the entire SendQ. Needed for access to the buffer or
	// to shutdown the queue.
	mutex sync.Mutex

	// Underlying io.WriteCloser to which writes will be buffered.
	writer io.WriteCloser

	// Whether all writing operations have been suspended.
	shutdown bool

	// Whether to shutdown sending and close the writer after the buffer
	// is flushed (emptied).
	flushAndClose bool

	// The actual byte buffer.
	buf []byte

	// Current position and overall buffer size.
	pos, size int

	// Condition which signals data is available for writing. The sending
	// loop will block on this if no data is available to write.
	dataAvailable *sync.Cond

	// Asynchronous error reporting channel.
	err chan error

	name string
}

func NewSendQ(writer io.WriteCloser, n int, wg *sync.WaitGroup) *SendQ {
	sq := &SendQ{
		writer: writer,
		pos:    0,
		size:   n,
		buf:    make([]byte, n),
		err:    make(chan error, 1),
		name:   "sq:unnamed",
	}
	sq.dataAvailable = sync.NewCond(&sq.mutex)
	wg.Add(1)
	go sq.loop(wg)
	return sq
}

func (sq *SendQ) Write(p []byte) (int, error) {
	// Writing requires the SendQ lock.
	sq.mutex.Lock()
	defer sq.mutex.Unlock()

	// If already shut down, don't bother.
	if sq.shutdown || sq.flushAndClose {
		goto done
	}

	// Determine if the pending write will fit.
	if sq.size-sq.pos < len(p) {
		// No capacity - bail out with an error.
		sq.flagError(SendQFull)
		goto done
	}

	// There is space. Copy len(p) bytes into the buffer.
	copy(sq.buf[sq.pos:], p)
	sq.pos += len(p)

	// The data is now available to be picked up by the sending loop.
	sq.dataAvailable.Signal()

done:
	// All Write() operations succeed. Errors are handled asynchronously.
	return len(p), nil
}

// Close the writer after all data has been flushed.
func (sq *SendQ) FlushAndClose() {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()

	sq.flushAndClose = true

	// Signal data available.
	sq.dataAvailable.Signal()
}

func (sq *SendQ) Close() {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()

	// Shutdown the sending loop.
	sq.shutdown = true
	sq.dataAvailable.Signal()

	// And close the underlying io.WriteCloser. Could be called simultaneously
	// with Write() which may return with an error. This error will be ignored.
	sq.writer.Close()
}

// Raises an error and shuts down the queue. Assumes the lock is held.
func (sq *SendQ) flagError(err error) {
	// Errors should only happen when not shutdown.
	if sq.shutdown {
		panic("Error raised when SendQ is already shutdown.")
	}
	sq.shutdown = true
	// Safe because the channel is buffered for one error. There should
	// never be a case where two errors are raised - the first should
	// cause a shutdown.
	sq.err <- err
	close(sq.err)

	// Need to signal the loop to wake up and shut itself down.
	sq.dataAvailable.Signal()

	// Close the underlying writer.
	sq.writer.Close()
}

func (sq *SendQ) ErrChan() <-chan error {
	return sq.err
}

// Main sending loop (run asynchronously). Waits for data to be available in
// the buffer and sends it to the writer. Exits whenever shutdown is set.
func (sq *SendQ) loop(wg *sync.WaitGroup) {
	defer wg.Done()
	sq.mutex.Lock()
	for {
		// Lock is held at the start of a loop iteration.
		// Await data or shutdown flag.
		for !sq.shutdown && !sq.flushAndClose && sq.pos == 0 {
			// Gives up lock until data becomes available.
			sq.dataAvailable.Wait()

			// In theory, data is always available here because there are no
			// other consumers. Best practices dictate that the condition be
			// checked here in a loop, though.
		}

		// Clean shutdown if requested.
		if sq.shutdown {
			sq.mutex.Unlock()
			return
		}

		if sq.flushAndClose && sq.pos == 0 {
			sq.shutdown = true
			sq.writer.Close()
			sq.mutex.Unlock()
			return
		}

		// Extract data to be sent in a slice.
		send := sq.buf[:sq.pos]

		// The actual send operation takes place outside the lock.
		sq.mutex.Unlock()
		n, err := sq.writer.Write(send)
		sq.mutex.Lock()

		// While the lock was released, a shutdown may have taken place.
		if sq.shutdown {
			sq.mutex.Unlock()
			return
		}

		// Flag any error if it occurred.
		if err != nil {
			sq.flagError(err)
			sq.mutex.Unlock()
			return
		}

		// The write was good. Move data in the buffer accordingly.
		copy(sq.buf, sq.buf[n:])
		sq.pos -= n

		// Lock is held at the end of a loop iteration.
	}
}
