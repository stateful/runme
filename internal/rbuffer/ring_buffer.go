package rbuffer

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var ErrClosed = errors.New("buffer closed")

type RingBuffer struct {
	mu         sync.Mutex
	buf        []byte
	size       int
	r          int // next position to read
	w          int // next position to write
	isFull     bool
	writeTrims *atomic.Int64
	closed     *atomic.Bool
	close      chan struct{}
	more       chan struct{}
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf:        make([]byte, size),
		size:       size,
		writeTrims: &atomic.Int64{},
		closed:     &atomic.Bool{},
		close:      make(chan struct{}),
		more:       make(chan struct{}),
	}
}

func (b *RingBuffer) Close() error {
	if b.closed.CompareAndSwap(false, true) {
		close(b.close)
	}
	return nil
}

func (b *RingBuffer) Reset() {
	b.mu.Lock()
	b.r = 0
	b.w = 0
	b.mu.Unlock()
}

func (b *RingBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	b.mu.Lock()
	n, err = b.read(p)
	b.mu.Unlock()

	if err != nil && errors.Is(err, io.EOF) && !b.closed.Load() {
		select {
		case <-b.more:
		case <-b.close:
			return 0, io.EOF
		}
		return n, nil
	}

	return n, err
}

func (b *RingBuffer) read(p []byte) (n int, err error) {
	if b.w == b.r && !b.isFull {
		return 0, io.EOF
	}

	if b.w > b.r {
		n = b.w - b.r
		if n > len(p) {
			n = len(p)
		}
		copy(p, b.buf[b.r:b.r+n])
		b.r = (b.r + n) % b.size
		b.isFull = false
		return
	}

	n = b.size - b.r + b.w
	if n > len(p) {
		n = len(p)
	}

	if b.r+n <= b.size {
		copy(p, b.buf[b.r:b.r+n])
	} else {
		copy(p, b.buf[b.r:b.size])
		c1 := b.size - b.r
		c2 := n - c1
		copy(p[c1:], b.buf[0:c2])
	}
	b.r = (b.r + n) % b.size
	b.isFull = false

	return n, err
}

func (b *RingBuffer) Write(p []byte) (n int, err error) {
	if b.closed.Load() {
		return 0, ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}

	b.mu.Lock()
	n, err = b.write(p)
	b.mu.Unlock()

	select {
	case b.more <- struct{}{}:
	default:
	}

	return n, err
}

func (b *RingBuffer) write(p []byte) (n int, err error) {
	if len(p) > b.size {
		p = p[len(p)-b.size:]
		b.writeTrims.Add(1)
	}

	var avail int
	if b.w >= b.r {
		avail = b.size - b.w + b.r
	} else {
		avail = b.r - b.w
	}

	n = len(p)

	if len(p) >= avail {
		b.isFull = true
		b.writeTrims.Add(1)
		b.r = b.w
		c := copy(b.buf[b.w:], p)
		b.w = copy(b.buf[0:], p[c:])
		return n, nil
	}

	if b.w >= b.r {
		c1 := b.size - b.w
		if c1 >= n {
			copy(b.buf[b.w:], p)
			b.w += n
		} else {
			copy(b.buf[b.w:], p[:c1])
			c2 := n - c1
			copy(b.buf[0:], p[c1:])
			b.w = c2
		}
	} else {
		copy(b.buf[b.w:], p)
		b.w += n
	}

	if b.w == b.size {
		b.isFull = true
		b.w = 0
	} else {
		b.isFull = false
	}

	return n, err
}

func (b *RingBuffer) Trims() int64 {
	return b.writeTrims.Load()
}
