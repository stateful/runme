package sbuffer

import (
	"bytes"
	"sync"
)

type Buffer struct {
	// +checklocks:mu
	b  *bytes.Buffer
	mu *sync.RWMutex
}

func New(buf []byte) *Buffer {
	return &Buffer{
		b:  bytes.NewBuffer(buf),
		mu: &sync.RWMutex{},
	}
}

func (b *Buffer) Bytes() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.b.Bytes()
}

func (b *Buffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.b.String()
}

func (b *Buffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Read(p)
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *Buffer) WriteString(s string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.WriteString(s)
}
