package rbuffer

import (
	"bytes"
	"crypto/rand"
	"io"
	mathRand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

func assertRead(t *testing.T, b *RingBuffer, expected []byte) {
	t.Helper()

	got := make([]byte, len(expected))
	n, err := b.read(got)
	assert.Nil(t, err)
	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, got)
}

func assertWrite(t *testing.T, b *RingBuffer, data []byte) {
	t.Helper()

	expected := len(data)
	if expected > b.size {
		expected = b.size
	}
	n, err := b.Write(data)
	assert.Nil(t, err)
	assert.Equal(t, expected, n)
}

func TestRingBuffer(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		data := []byte("hello")
		buf := NewRingBuffer(10)
		assertWrite(t, buf, data)
		assertRead(t, buf, data)

		data = []byte("helloworld")
		assertWrite(t, buf, data)
		assertRead(t, buf, data)

		data = []byte("world")
		assertWrite(t, buf, data)
		assertRead(t, buf, data)

		data = []byte("HELLO123")
		assertWrite(t, buf, data)
		assertRead(t, buf, data)

		data = append(bytes.Repeat([]byte{1}, 10), bytes.Repeat([]byte{2}, 5)...)
		assertWrite(t, buf, data)
		assertRead(t, buf, data[5:])
		data = append(bytes.Repeat([]byte{1}, 25), bytes.Repeat([]byte{2}, 8)...)
		assertWrite(t, buf, data)
		assertRead(t, buf, data[23:])
	})

	t.Run("ExceedingInput", func(t *testing.T) {
		buf := NewRingBuffer(4567) // not a power of 2

		var g errgroup.Group

		g.Go(func() error {
			token := make([]byte, 8<<10)         // 8 KiB
			unwritten := int64((64 << 10) << 10) // 64 MiB

			for unwritten > 0 {
				c := mathRand.Intn(cap(token))
				if c > int(unwritten) {
					c = int(unwritten)
				}

				n, err := rand.Read(token[:c])
				if err != nil {
					return err
				}

				n, err = buf.Write(token[:n])
				if err != nil {
					return err
				}

				unwritten -= int64(n)
			}

			return buf.Close()
		})

		g.Go(func() error {
			_, err := io.Copy(io.Discard, buf)
			return err
		})

		assert.NoError(t, g.Wait())
	})
}

func TestRingBuffer_Parallel(t *testing.T) {
	buf := NewRingBuffer(10 * 1000 * 1000)

	var g errgroup.Group

	for i := 0; i < 100; i++ {
		g.Go(func() error {
			data := make([]byte, 1000)
			if _, err := rand.Read(data); err != nil {
				return err
			}
			_, err := buf.Write(data)
			return err
		})
	}

	for i := 0; i < 100; i++ {
		g.Go(func() error {
			data := make([]byte, 1000)
			_, err := buf.Read(data)
			return err
		})
	}

	assert.NoError(t, g.Wait())
}

func TestRingBuffer_Close(t *testing.T) {
	buf := NewRingBuffer(512)
	assert.NoError(t, buf.Close())
	assert.NoError(t, buf.Close())
	n, err := buf.Write([]byte{1})
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, ErrClosed)
	p := make([]byte, 32)
	n, err = buf.Read(p)
	assert.Equal(t, 0, n)
	assert.Error(t, err)
}
