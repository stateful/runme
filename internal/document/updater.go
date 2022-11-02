package document

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
)

var ErrIndexOutOfRange = errors.New("block index out of range")

type Updater struct {
	parsed *ParsedSource
}

func NewUpdater(parsed *ParsedSource) *Updater {
	return &Updater{
		parsed: parsed,
	}
}

func NewUpdaterWithSource(data []byte) *Updater {
	return &Updater{
		parsed: NewSource(data).Parse(),
	}
}

func (u *Updater) Parsed() *ParsedSource {
	return u.parsed
}

func (u *Updater) UpdateBlock(idx int, newSource string) error {
	blocks := u.parsed.Blocks()
	if idx >= len(blocks) {
		return ErrIndexOutOfRange
	}

	block := blocks[idx]
	start, stop := block.Start(), block.Stop()

	before := u.parsed.data[0:start]
	after := u.parsed.data[stop:]

	buf := bytes.NewBuffer(make([]byte, 0, len(before)+len(newSource)+len(after)))

	_, _ = buf.Write(before)
	_, _ = buf.WriteString(newSource)
	_, _ = buf.Write(after)

	u.parsed = NewSource(buf.Bytes()).Parse()

	return nil
}

func (u *Updater) Write(w io.Writer) error {
	_, err := w.Write(u.parsed.data)
	return errors.WithStack(err)
}
