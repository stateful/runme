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
	"go.uber.org/zap"
)

type Session struct {
	id             string
	done           chan struct{}
	err            error
	cmd            *exec.Cmd
	ptmx           *os.File
	ready          chan struct{}
	isReady        bool
	outputMu       sync.Mutex
	output         io.Writer
	prompt         []byte // detected prompt
	promptNotifier chan struct{}
	logger         *zap.Logger
}

func NewSession(
	prompt []byte,
	commandName string,
	logger *zap.Logger,
) (*Session, error) {
	s := Session{
		id:             xid.New().String(),
		done:           make(chan struct{}),
		ready:          make(chan struct{}),
		output:         io.Discard,
		prompt:         prompt,
		promptNotifier: make(chan struct{}, 1),
		logger:         logger,
	}

	s.cmd = exec.Command(commandName)
	s.cmd.Env = append(os.Environ(), "RUNMESHELL="+s.id)

	var err error
	s.ptmx, err = pty.Start(s.cmd)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to start shell in pty")
	}

	go func() {
		err := s.copyOutput()
		if err != nil && errors.Is(err, io.EOF) {
			err = nil
			close(s.done)
		}
		s.logger.With(zap.Error(err)).Info("finished copying from ptmx to outputs")
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
	if err := s.cmd.Process.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill command")
	}
	<-s.done
	return nil
}

type bufferedWriter struct {
	buf         []byte
	passthrough bool
	start       []byte
	end         []byte
	n           int
	err         error
	wr          io.Writer
}

func newBufferedWriterSize(w io.Writer, start, end []byte, size int) *bufferedWriter {
	if size <= 0 {
		size = 4096
	}
	if (len(start)+len(end))*2 > size {
		panic("size of buffer is too small")
	}
	return &bufferedWriter{
		buf:   make([]byte, size),
		start: start,
		end:   end,
		wr:    w,
	}
}

func (b *bufferedWriter) Available() int { return len(b.buf) - b.n }

func (b *bufferedWriter) Buffered() int { return b.n }

func (b *bufferedWriter) Reset(w io.Writer) {
	b.err = nil
	b.n = 0
	b.wr = w
}

func (b *bufferedWriter) Flush() error {
	if err := b.write(); err != nil {
		return err
	}
	if b.n == 0 {
		return nil
	}
	var (
		n   int
		err error
	)
	if b.passthrough {
		n, err = b.wr.Write(b.buf[0:b.n])
	} else {
		n = b.n
	}
	b.err = err
	b.n -= n
	return b.err
}

func (b *bufferedWriter) Write(p []byte) (nn int, err error) {
	for len(p) > b.Available() && b.err == nil {
		n := copy(b.buf[b.n:], p)
		b.n += n
		b.write()
		nn += n
		p = p[n:]
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}

func (b *bufferedWriter) write() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}

	var (
		n   int
		err error
	)

start:
	if b.passthrough {
		idx := bytes.Index(b.buf[0:b.n], b.end)
		if idx > -1 {
			n, err = b.wr.Write(b.buf[0:idx])
			if n < idx && err == nil {
				err = io.ErrShortWrite
			}
			n += len(b.end)
			b.passthrough = false
		} else {
			m := b.n - len(b.end) + 1
			n, err = b.wr.Write(b.buf[0:m])
			if n < m && err == nil {
				err = io.ErrShortWrite
			}
			copy(b.buf[0:], b.buf[m:])
		}
	} else {
		idx := bytes.Index(b.buf[0:b.n], b.start)
		if idx > -1 {
			s := idx + len(b.start)
			copy(b.buf[0:], b.buf[s:])
			b.n -= s
			b.passthrough = true
			goto start
		} else {
			copy(b.buf[0:], b.buf[b.n-len(b.start):])
			n = b.n - len(b.start)
		}
	}
	b.n -= n
	if err != nil {
		b.err = err
	}
	return nil
}

func (s *Session) Execute(command []byte, output io.Writer) (int, error) {
	s.logger.Info("executing command", zap.ByteString("command", command))

	w := newBufferedWriterSize(
		output,
		append(command, '\r', '\n'),
		append([]byte{'\r', '\n'}, s.prompt...),
		-1,
	)

	s.logger.Info("new buffered writer", zap.ByteString("command", command), zap.ByteString("prompt", s.prompt))

	err := s.execute(command, w)
	if err != nil {
		s.logger.Error("failed to execute command", zap.Error(err))
		return -1, err
	}
	if err := w.Flush(); err != nil {
		s.logger.Error("failed to flush buffered writer", zap.Error(err))
		return -1, err
	}

	s.logger.Info("getting exit code for command", zap.ByteString("command", command))

	code, err := s.getExitCode()
	if err != nil {
		s.logger.Error("failed to get exit code", zap.Error(err))
	}
	return code, err
}

var crlf = []byte{'\r', '\n'}

func (s *Session) getExitCode() (int, error) {
	exitCodeCommand := []byte("echo $?")

	var buf bytes.Buffer
	err := s.execute(exitCodeCommand, &buf)
	if err != nil {
		return -1, err
	}

	data := buf.Bytes()

	s.logger.Info("got output for exit code command", zap.ByteString("data", data))

	// Remove the command itself if it is a prefix.
	data = bytes.TrimPrefix(data, exitCodeCommand)
	// Remove any CRLF prefix.
	for bytes.HasPrefix(data, crlf) {
		data = data[len(crlf):]
	}
	// Remove all suffixes.
	for lastIdx := len(data); lastIdx > -1; lastIdx = bytes.LastIndex(data, crlf) {
		data = data[:lastIdx]
	}

	s.logger.Info("extracted exit code from output", zap.ByteString("code", data))

	exitCode, err := strconv.ParseInt(string(data), 10, 8)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	return int(exitCode), nil
}

func (s *Session) execute(command []byte, w io.Writer) error {
	<-s.ready

	if command[len(command)-1] != '\n' {
		command = append(command, '\n')
	}

	var prevOutput io.Writer

	s.outputMu.Lock()
	prevOutput = s.output
	s.output = io.MultiWriter(prevOutput, w)
	s.outputMu.Unlock()

	_, err := s.ptmx.Write(command)
	if err != nil {
		return errors.WithStack(err)
	}

	<-s.promptNotifier
	s.outputMu.Lock()
	s.output = prevOutput
	s.outputMu.Unlock()

	return nil
}

func (s *Session) copyOutput() error {
	buf := make([]byte, 32*1024)
	if cap(buf) < len(s.prompt) {
		return errors.Errorf("prompt is longer than buffer size (%d vs %d)", len(s.prompt), cap(buf))
	}
	var err error
	for {
		nr, er := s.ptmx.Read(buf)
		s.logger.Info("read from ptmx", zap.ByteString("data", buf[0:nr]), zap.Error(err))
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
			s.logger.Info("detected prompt in the output", zap.Bool("ready", s.isReady))
			if !s.isReady {
				s.isReady = true
				close(s.ready)
			} else {
				select {
				case s.promptNotifier <- struct{}{}:
				default:
				}
			}
		}
	}
	return err
}
