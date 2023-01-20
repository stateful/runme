package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ringBuffer(t *testing.T) {
	assertRead := func(t *testing.T, b *ringBuffer, expected []byte) {
		got := make([]byte, len(expected))
		n, err := b.read(got)
		assert.Nil(t, err)
		assert.Equal(t, len(expected), n)
		assert.Equal(t, expected, got)
	}

	t.Run("basic", func(t *testing.T) {
		data := []byte("hello")
		buf := newRingBuffer(10)
		n, err := buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, len(data), n)
		assertRead(t, buf, data)
	})

	t.Run("overwriting", func(t *testing.T) {
		data := []byte("hello")
		buf := newRingBuffer(5)
		n, err := buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, len(data), n)

		data = []byte("world")
		n, err = buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, len(data), n)
		assertRead(t, buf, data)
	})

	t.Run("wrapping", func(t *testing.T) {
		data := []byte("hello")
		buf := newRingBuffer(10)
		n, err := buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, len(data), n)

		data = []byte("123world")
		n, err = buf.Write(data)
		assert.Nil(t, err)
		assert.Equal(t, len(data), n)
		assertRead(t, buf, data)
	})
}
