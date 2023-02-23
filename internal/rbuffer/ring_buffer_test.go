package rbuffer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertRead(t *testing.T, b *RingBuffer, expected []byte) {
	got := make([]byte, len(expected))
	n, err := b.read(got)
	assert.Nil(t, err)
	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, got)
}

func assertWrite(t *testing.T, b *RingBuffer, data []byte) {
	n, err := b.Write(data)
	assert.Nil(t, err)
	if len(data) > b.size {
		n = b.size
	}
	assert.Equal(t, len(data), n)
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
	})

	t.Run("ExceedingInput", func(t *testing.T) {
		data := append(
			bytes.Repeat([]byte{'1'}, 512),
			bytes.Repeat([]byte{'2'}, 512)...,
		)
		buf := NewRingBuffer(512)
		n, err := buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, buf.size, n)
		assertRead(t, buf, data[1024-buf.size:])
	})

	t.Run("ExceedingInputTerminates", func(t *testing.T) {
		data := append(
			bytes.Repeat([]byte{'1'}, 1024),
			bytes.Repeat([]byte{'2'}, 25*1024)...,
		)

		buf := NewRingBuffer(10 * 1024)
		n, err := buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, buf.size, n)

		expected := bytes.Repeat([]byte{'2'}, buf.size)
		iterationCount := 0

		for {
			got := make([]byte, len(expected))

			n, err = buf.read(got)

			if n == 0 || err != nil {
				break
			}

			assert.Equal(t, expected, got)

			iterationCount++

			if iterationCount > 1 {
				break
			}
		}

		assert.Equal(t, 1, iterationCount)
	})
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
