package kernel

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/creack/pty"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	xpty "github.com/stateful/runme/internal/pty"
	"golang.org/x/term"
)

type Session struct {
	id             string
	done           chan struct{}
	err            error
	ptmx           *os.File
	cancelResize   xpty.CancelFn
	stdinOldState  *term.State
	outputMu       sync.Mutex
	output         io.Writer
	prompt         []byte // detected prompt
	promptNotifier chan struct{}
}

func NewSession(
	prompt []byte,
	commandName string,
) (*Session, error) {
	s := Session{
		id:             xid.New().String(),
		done:           make(chan struct{}),
		output:         os.Stdout,
		prompt:         prompt,
		promptNotifier: make(chan struct{}, 1),
	}

	c := exec.Command(commandName)
	c.Env = append(os.Environ(), "RUNMESHELL="+s.id)

	var err error
	s.ptmx, err = pty.Start(c)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to start shell in pty")
	}

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

	go func() {
		err := s.copy()
		if err != nil && errors.Is(err, io.EOF) {
			err = nil
			close(s.done)
		}
		if err != nil {
			panic(err)
		}
		s.setErrorf(err, "failed to copy from ptmx to stdout")
	}()

	return &s, nil
}

func (s *Session) setErrorf(err error, msg string, args ...interface{}) {
	if err != nil {
		s.err = multierror.Append(s.err, errors.WithMessagef(err, msg, args...))
	}
}

func (s *Session) Err() error {
	return s.err
}

func (s *Session) ID() string {
	return s.id
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) Destroy() error {
	s.cancelResize()

	if err := term.Restore(int(os.Stdin.Fd()), s.stdinOldState); err != nil {
		return errors.WithStack(err)
	}

	if err := s.ptmx.Close(); err != nil {
		return errors.WithStack(err)
	}

	<-s.done

	return nil
}

func (s *Session) Execute(command []byte, stdout io.Writer) (int, error) {
	err := s.execute(command, io.MultiWriter(os.Stdout, stdout))
	if err != nil {
		return -1, err
	}

	var buf bytes.Buffer
	err = s.execute([]byte("echo $?"), &buf)
	if err != nil {
		return -1, err
	}
	exitCode, err := strconv.ParseInt(buf.String(), 10, 8)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	return int(exitCode), nil
}

func (s *Session) execute(command []byte, w io.Writer) error {
	var oldOutput io.Writer

drain:
	for {
		select {
		case <-s.promptNotifier:
		default:
			s.outputMu.Lock()
			oldOutput = s.output
			s.output = w
			s.outputMu.Unlock()
			break drain
		}
	}

	_, err := s.ptmx.Write(command)
	if err != nil {
		return errors.WithStack(err)
	}
	// _, _ = s.ptmx.Write([]byte("\r\n"))

	<-s.promptNotifier

	s.outputMu.Lock()
	s.output = oldOutput
	s.outputMu.Unlock()

	return nil
}

func (s *Session) copy() error {
	buf := make([]byte, 32*1024)
	if cap(buf) < len(s.prompt) {
		return errors.Errorf("prompt is longer than buffer size (%d vs %d)", len(s.prompt), cap(buf))
	}
	var err error
	for {
		nr, er := s.ptmx.Read(buf)
		if nr > 0 {
			s.outputMu.Lock()
			nw, ew := s.output.Write(buf[0:nr])
			s.outputMu.Unlock()
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = errors.WithStack(io.ErrShortWrite)
				break
			}
		}
		if er != nil {
			err = errors.WithStack(er)
			break
		}
		if bytes.Contains(buf[0:nr], s.prompt) {
			select {
			case s.promptNotifier <- struct{}{}:
			default:
			}
		}
	}
	return err
}
