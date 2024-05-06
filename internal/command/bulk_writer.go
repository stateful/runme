package command

import (
	"io"

	"github.com/pkg/errors"
)

type bulkWriter struct {
	io.Writer
	n   int
	err error
}

func (w *bulkWriter) Done() (int, error) {
	return w.n, w.err
}

func (w *bulkWriter) Write(d []byte) {
	if w.err != nil {
		return
	}
	n, err := w.Writer.Write(d)
	w.n += n
	w.err = errors.WithStack(err)
}

func (w *bulkWriter) WriteByte(c byte) {
	w.Write([]byte{c})
}

func (w *bulkWriter) WriteString(s string) {
	w.Write([]byte(s))
}
