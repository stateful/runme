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
	done           chan struct{} // notifies when Session's process is finished
	err            error         // error from background processing, e.g. copy output
	cmd            *exec.Cmd
	ptmx           *os.File
	isReady        bool
	ready          chan struct{} // notifies when the process is ready; for shells upon observing the first prompt
	outputMu       sync.Mutex    // for output; outputCopyFunc is immutable after initialization
	output         io.Writer
	outputCopyFunc func() error
	outputRaw      bool
	prompt         []byte
	promptNotifier chan struct{}
	logger         *zap.Logger
}

func NewSession(
	prompt []byte,
	commandName string,
	raw bool,
	logger *zap.Logger,
) (*Session, error) {
	s := Session{
		id:             xid.New().String(),
		done:           make(chan struct{}),
		ready:          make(chan struct{}),
		output:         io.Discard,
		outputRaw:      raw,
		prompt:         prompt,
		promptNotifier: make(chan struct{}, 1),
		logger:         logger,
	}

	s.outputCopyFunc = s.copyOuput

	s.cmd = exec.Command(commandName)
	s.cmd.Env = append(os.Environ(), "RUNMESHELL="+s.id)

	var err error
	s.ptmx, err = pty.Start(s.cmd)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to start shell in pty")
	}

	go func() {
		defer close(s.done)
		err := s.outputCopyFunc()
		if err != nil && errors.Is(err, io.EOF) {
			err = nil
		}
		s.logger.With(zap.Error(err)).Info("finished copying from ptmx to outputs")
		s.setErrorf(err, "failed to copy from ptmx to stdout")
	}()

	return &s, nil
}

func (s *Session) ID() string {
	return s.id
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) Err() error {
	return s.err
}

func (s *Session) Wait() error {
	<-s.done
	return s.err
}

func (s *Session) setErrorf(err error, msg string, args ...interface{}) {
	if err != nil {
		s.err = multierror.Append(s.err, errors.WithMessagef(err, msg, args...))
	}
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
	code, err := s.exitCode()
	if err != nil {
		s.logger.Error("failed to get exit code", zap.Error(err))
	}
	return code, err
}

var (
	crlf            = []byte{'\r', '\n'}
	lf              = []byte{'\n'}
	exitCodeCommand = []byte("echo $?")
)

func cleanExitCodeOutput(data []byte) []byte {
	data = dropANSIEscape(data)
	data = bytes.TrimPrefix(data, crlf)
	data = bytes.TrimPrefix(data, exitCodeCommand)
	data = bytes.TrimPrefix(data, crlf)
	data = dropCRPrefix(data)
	for lastIdx := len(data); lastIdx > -1; lastIdx = bytes.LastIndex(data, crlf) {
		data = data[:lastIdx]
	}
	return data
}

func (s *Session) exitCode() (int, error) {
	var buf bytes.Buffer
	err := s.execute(exitCodeCommand, &buf)
	if err != nil {
		return -1, err
	}

	data := buf.Bytes()
	s.logger.Info("output for exit code command", zap.ByteString("data", data))
	data = cleanExitCodeOutput(data)
	s.logger.Info("extracted exit code from output", zap.ByteString("code", data))

	exitCode, err := strconv.ParseInt(string(data), 10, 8)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	return int(exitCode), nil
}

func (s *Session) execute(command []byte, w io.Writer) error {
	<-s.ready // wait for the first prompt before executing anything

	// command must end with a new line, otherwise it won't be executed
	if command[len(command)-1] != '\n' {
		command = append(command, '\n')
	}

	var prevOutput io.Writer

	s.outputMu.Lock()
	prevOutput = s.output
	s.output = io.MultiWriter(prevOutput, w)
	s.outputMu.Unlock()

	// collect N prompts; depending on the number of new lines in command
	prompts := make(chan struct{})
	go func() {
		n := bytes.Count(command, lf)
		for i := 0; i < n; i++ {
			<-s.promptNotifier
		}
		close(prompts)
	}()

	_, err := s.ptmx.Write(command)
	if err != nil {
		return errors.WithStack(err)
	}

	<-prompts

	s.outputMu.Lock()
	s.output = prevOutput
	s.outputMu.Unlock()

	return nil
}

func (s *Session) copyOuput() error {
	scanner := bufio.NewScanner(s.ptmx)
	scanner.Split(scanLines(s.prompt))

	for scanner.Scan() {
		line := scanner.Bytes()
		s.logger.Debug("read line to copy", zap.ByteString("line", line))

		if !s.outputRaw {
			line = dropCRPrefix(dropANSIEscape(line))
			s.logger.Debug("line after clean up", zap.ByteString("line", line))
		}

		if len(line) == 0 {
			continue
		}

		hasPrompt := false

		if bytes.HasSuffix(line, s.prompt) {
			s.logger.Info("detected prompt in the line", zap.Bool("ready", s.isReady))

			hasPrompt = true
			s.promptDetected()

			if !s.outputRaw {
				continue
			}
		}

		s.outputMu.Lock()
		out := s.output
		s.outputMu.Unlock()

		_, err := out.Write(line)
		if err != nil {
			return errors.WithStack(err)
		}

		if !hasPrompt {
			_, err = out.Write(crlf)
			if err != nil {
				return errors.WithStack(err)
			}
		} else {
			_, err = out.Write([]byte{' '})
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	return errors.WithStack(scanner.Err())
}

func (s *Session) promptDetected() {
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

func scanLines(prompt []byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		return scanLinesPrompt(data, atEOF, prompt)
	}
}

func scanLinesPrompt(data []byte, atEOF bool, prompt []byte) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	promptIdx := bytes.Index(data, prompt)
	newLineIdx := bytes.IndexByte(data, '\n')
	if promptIdx >= 0 && (newLineIdx < 0 || promptIdx < newLineIdx) {
		m := promptIdx + len(prompt)
		if len(data) > m && data[m] == ' ' {
			m++
		}
		return m, data[0 : promptIdx+len(prompt)], nil
	} else if newLineIdx >= 0 {
		// We have a full newline-terminated line.
		return newLineIdx + 1, dropCR(data[0:newLineIdx]), nil
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

func dropCRPrefix(data []byte) []byte {
	if len(data) > 0 && data[0] == '\r' {
		return data[1:]
	}
	return data
}
