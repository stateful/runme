package runnerv2service

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

const (
	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 16 * 1024 * 1024 // 16 MiB
)

// buffer is a thread-safe buffer that returns EOF
// only when it's closed.
type buffer struct {
	mu *sync.Mutex
	// +checklocks:mu
	b      *bytes.Buffer
	closed *atomic.Bool
	close  chan struct{}
	more   chan struct{}
}

var _ io.WriteCloser = (*buffer)(nil)

func newBuffer(size int) *buffer {
	return &buffer{
		mu:     &sync.Mutex{},
		b:      bytes.NewBuffer(make([]byte, 0, size)),
		closed: &atomic.Bool{},
		close:  make(chan struct{}),
		more:   make(chan struct{}),
	}
}

func (b *buffer) Write(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, errors.New("closed")
	}

	b.mu.Lock()
	n, err := b.b.Write(p)
	b.mu.Unlock()

	select {
	case b.more <- struct{}{}:
	default:
	}

	return n, err
}

func (b *buffer) Close() error {
	if b.closed.CompareAndSwap(false, true) {
		close(b.close)
	}
	return nil
}

func (b *buffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.b.Read(p)
	b.mu.Unlock()

	if err != nil && errors.Is(err, io.EOF) && !b.closed.Load() {
		select {
		case <-b.more:
		case <-b.close:
			return n, io.EOF
		}
		return n, nil
	}

	return n, err
}
