package kernel

import (
	"bufio"
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
		defer close(s.done)
		err := s.copyOutput()
		if err != nil && errors.Is(err, io.EOF) {
			err = nil
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

func (s *Session) Execute(command []byte, output io.Writer) (int, error) {
	s.logger.Info("executing command", zap.ByteString("command", command))

	err := s.execute(command, output)
	if err != nil {
		s.logger.Error("failed to execute command", zap.Error(err))
		return -1, err
	}

	s.logger.Info("getting exit code for command", zap.ByteString("command", command))

	code, err := s.getExitCode()
	if err != nil {
		s.logger.Error("failed to get exit code", zap.Error(err))
	}
	return code, err
}

var (
	crlf = []byte{'\r', '\n'}
	lf   = []byte{'\n'}
)

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

	expectedPrompts := bytes.Count(command, lf)
	for i := 0; i < expectedPrompts; i++ {
		<-s.promptNotifier
	}
	s.outputMu.Lock()
	s.output = prevOutput
	s.outputMu.Unlock()

	return nil
}

func (s *Session) copyOutput() error {
	scanner := bufio.NewScanner(s.ptmx)
	scanner.Split(scanLinesPrompt(s.prompt))

	for scanner.Scan() {
		line := scanner.Bytes()

		s.logger.Info("read line", zap.ByteString("line", line))

		if len(line) == 0 {
			continue
		}

		if bytes.HasSuffix(line, s.prompt) {
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
			continue
		}

		_, err := s.output.Write(line)
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = s.output.Write(crlf)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return errors.WithStack(scanner.Err())
}

func scanLinesPrompt(prompt []byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		return scanLines(data, atEOF, prompt)
	}
}

func scanLines(data []byte, atEOF bool, prompt []byte) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, prompt); i >= 0 {
		m := i + len(prompt)
		if len(data) >= m && data[m] == ' ' {
			m++
		}
		return m, data[0 : i+len(prompt)], nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}
