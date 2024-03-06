package term

import (
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

type Term interface {
	In() io.Reader
	Out() io.Writer
	ErrOut() io.Writer
	IsTTY() bool
	Size() (int, int, error)
}

func System() Term {
	return &osTerm{
		in:     os.Stdin,
		out:    os.Stdout,
		errOut: os.Stderr,
		isTTY:  isTerminal(os.Stdout),
	}
}

func FromIO(in io.Reader, out, errOut io.Writer) Term {
	inF, inOk := in.(*os.File)
	outF, outOk := out.(*os.File)
	errOutF, errOutOk := errOut.(*os.File)

	if !inOk || !outOk || !errOutOk {
		return &ioTerm{
			in:     in,
			out:    out,
			errOut: errOut,
		}
	}

	return &osTerm{
		in:     inF,
		out:    outF,
		errOut: errOutF,
		isTTY:  isTerminal(outF),
	}
}

type osTerm struct {
	in     *os.File
	out    *os.File
	errOut *os.File
	isTTY  bool
}

func (t *osTerm) In() io.Reader     { return t.in }
func (t *osTerm) Out() io.Writer    { return t.out }
func (t *osTerm) ErrOut() io.Writer { return t.errOut }

func (t *osTerm) IsTTY() bool { return t.isTTY }

func (t *osTerm) Size() (int, int, error) {
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

type ioTerm struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (t *ioTerm) In() io.Reader     { return t.in }
func (t *ioTerm) Out() io.Writer    { return t.out }
func (t *ioTerm) ErrOut() io.Writer { return t.errOut }

func (t *ioTerm) IsTTY() bool { return false }

func (t *ioTerm) Size() (int, int, error) {
	return -1, -1, errors.New("not a tty")
}
