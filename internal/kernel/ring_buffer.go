package kernel

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

type ringBuffer struct {
	mu     sync.Mutex
	buf    []byte
	size   int
	r      int // next position to read
	w      int // next position to write
	isFull bool
	state  atomic.Bool
	more   chan struct{}
	closed chan struct{}
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

func (b *ringBuffer) Reset() {
	b.mu.Lock()
	b.r = 0
	b.w = 0
	b.mu.Unlock()
}

func (b *ringBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	b.mu.Lock()
	n, err = b.read(p)
	b.mu.Unlock()

	if err != nil && errors.Is(err, io.EOF) && b.state.Load() {
		select {
		case <-b.more:
		case <-b.closed:
			return 0, io.EOF
		}
		return n, nil
	}

	return n, err
}

func (b *ringBuffer) read(p []byte) (n int, err error) {
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
		return
	}

	n = b.size - b.r + b.w
	if n > len(p) {
		n = len(p)
	}

	if b.r+n <= b.size || b.isFull {
		copy(p, b.buf[b.r:b.r+n])
	} else {
		copy(p, b.buf[b.r:b.size])
		c1 := b.size - b.r
		c2 := n - c1
		copy(p[c1:], b.buf[0:c2])
	}
	b.r = (b.r + n) % b.size

	return n, err
}

func (b *ringBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(p) > b.size {
		return 0, errors.New("buffer is too small")
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

func (b *ringBuffer) write(p []byte) (n int, err error) {
	var avail int
	if b.w >= b.r {
		avail = b.size - b.w + b.r
	} else {
		avail = b.r - b.w
	}

	n = len(p)

	if len(p) > avail {
		b.isFull = false
		b.r = b.w
		copy(b.buf[b.w:], p[:b.size-b.w])
		b.w = copy(b.buf[0:], p[b.size-b.w:])
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
