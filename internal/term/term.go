package term

import (
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

type Term struct {
	in     *os.File
	out    *os.File
	errOut *os.File
	isTTY  bool
}

func System() *Term {
	return &Term{
		in:     os.Stdin,
		out:    os.Stdout,
		errOut: os.Stderr,
		isTTY:  isTerminal(os.Stdout),
	}
}

func (t *Term) In() io.Reader     { return t.in }
func (t *Term) Out() io.Writer    { return t.out }
func (t *Term) ErrOut() io.Writer { return t.errOut }

func (t *Term) IsTTY() bool { return t.isTTY }

func (t *Term) Size() (int, int, error) {
	if !t.isTTY {
		return -1, -1, errors.New("not a tty")
	}
	return terminalSize(t.out)
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func terminalSize(f *os.File) (int, int, error) {
	return term.GetSize(int(f.Fd()))
}
