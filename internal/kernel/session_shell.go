package kernel

import (
	"io"
	"os"

	"github.com/pkg/errors"
	xpty "github.com/stateful/runme/internal/pty"
	"go.uber.org/zap"
	"golang.org/x/term"
)

type ShellSession struct {
	*Session

	cancelResize  xpty.CancelFn
	stdinOldState *term.State
}

func NewShellSession(
	prompt []byte,
	commandName string,
) (*ShellSession, error) {
	sess, err := NewSession(prompt, commandName, zap.NewNop())
	if err != nil {
		return nil, err
	}

	s := ShellSession{Session: sess}
	s.logger = zap.NewNop()
	s.output = os.Stdout

	s.cancelResize = xpty.ResizeOnSig(s.ptmx)

	// Set stdin in raw mode.
	s.stdinOldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to put stdin in raw mode")
	}

	go func() {
		// TODO: cancel this goroutine when ptmx is closed
		_, err := io.Copy(s.ptmx, os.Stdin)
		s.setErrorf(err, "failed to copy stdin to ptmx")
	}()

	return &s, nil
}

func (s *ShellSession) Destroy() error {
	s.cancelResize()

	if err := term.Restore(int(os.Stdin.Fd()), s.stdinOldState); err != nil {
		return errors.WithStack(err)
	}

	return s.Session.Destroy()
}
