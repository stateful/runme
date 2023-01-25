package runner

import (
	"bytes"
	"io"
	"sync"
)

type safeBuffer struct {
	buf *bytes.Buffer
	n   int
	mx  sync.Mutex
}

var _ io.ReadWriter = (*safeBuffer)(nil)

func (w *safeBuffer) Write(p []byte) (n int, err error) {
	w.mx.Lock()
	n, err = w.buf.Write(p)
	w.n += n
	w.mx.Unlock()
	return
}

func (w *safeBuffer) Read(p []byte) (n int, err error) {
	w.mx.Lock()
	n, err = w.buf.Read(p)
	w.n -= n
	w.mx.Unlock()
	return
}

func (w *safeBuffer) Bytes() []byte {
	w.mx.Lock()
	data := w.buf.Next(w.n)
	w.mx.Unlock()
	return data
}
