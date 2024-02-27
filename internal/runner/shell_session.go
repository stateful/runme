package runner

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	xpty "github.com/stateful/runme/v3/internal/pty"
	"github.com/stateful/runme/v3/internal/ulid"
	"golang.org/x/term"
)

type ShellSession struct {
	id            string
	cmd           *exec.Cmd
	ptmx          *os.File
	cancelResize  xpty.CancelFn
	stdinOldState *term.State
	done          chan struct{}
	mu            sync.Mutex
	err           error
}

func NewShellSession(command string) (*ShellSession, error) {
	id := ulid.GenerateID()

	cmd := exec.Command(command)
	cmd.Env = append(os.Environ(), "RUNMESHELL="+id)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cancelResize := xpty.ResizeOnSig(ptmx)
	// Set stdin in raw mode.
	stdinOldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to put stdin in raw mode")
	}

	s := &ShellSession{
		id:            id,
		cmd:           cmd,
		ptmx:          ptmx,
		cancelResize:  cancelResize,
		stdinOldState: stdinOldState,
		done:          make(chan struct{}),
	}

	go func() {
		defer close(s.done)
		_, err := io.Copy(os.Stdout, ptmx)
		if err != nil && errors.Is(err, io.EOF) {
			err = nil
		}
		s.setErrorf(err, "failed to copy ptmx to stdout")
	}()

	go func() {
		// TODO: cancel this goroutine when ptmx is closed
		_, err := io.Copy(ptmx, os.Stdin)
		s.setErrorf(err, "failed to copy stdin to ptmx")
	}()

	return s, nil
}

func (s *ShellSession) ID() string {
	return s.id
}

func (s *ShellSession) Close() error {
	s.cancelResize()

	if err := term.Restore(int(os.Stdin.Fd()), s.stdinOldState); err != nil {
		return errors.WithStack(err)
	}

	if err := s.cmd.Process.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill command")
	}
	<-s.done
	return nil
}

func (s *ShellSession) Done() <-chan struct{} {
	return s.done
}

func (s *ShellSession) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *ShellSession) Send(data []byte) error {
	_, err := s.ptmx.Write(data)
	return errors.Wrap(err, "failed to write data to ptmx")
}

func (s *ShellSession) setErrorf(err error, msg string, args ...interface{}) {
	if s.err != nil || err == nil {
		return
	}
	s.mu.Lock()
	s.err = errors.Wrapf(err, msg, args...)
	s.mu.Unlock()
}
