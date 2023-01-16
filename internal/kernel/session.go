package kernel

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"github.com/stateful/runme/expect"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type limitedBuffer struct {
	*bytes.Buffer // TODO: switch to a ring buffer
	mu            sync.Mutex
	state         *atomic.Uint32
	more          chan struct{}
	closed        chan struct{}
}

func (b *limitedBuffer) Close() error {
	b.state.Store(1)
	close(b.closed)
	return nil
}

func (b *limitedBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.Buffer.Read(p)
	b.mu.Unlock()
	if err != nil && errors.Is(err, io.EOF) && b.state.Load() == 0 {
		select {
		case <-b.more:
		case <-b.closed:
			return 0, io.EOF
		}
		return n, nil
	}
	return n, err
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	n, _ := b.Buffer.Write(p)
	b.mu.Unlock()
	select {
	case b.more <- struct{}{}:
	default:
	}
	return n, nil
}

type session struct {
	id       string
	prompt   string
	promptRe *regexp.Regexp
	ptmx     *os.File
	expctr   expect.Expecter
	output   *limitedBuffer
	done     chan struct{}
	err      error
	logger   *zap.Logger
}

func newSession(command, prompt string, logger *zap.Logger) (*session, []byte, error) {
	promptRe, err := compileLiteralRegexp(prompt)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	buf := &limitedBuffer{
		Buffer: bytes.NewBuffer(nil),
		state:  atomic.NewUint32(0),
		more:   make(chan struct{}),
		closed: make(chan struct{}),
	}

	_, ptmx, expctr, cmdErr, err := spawnPty(
		command,
		-1,
		expect.Tee(buf),
		expect.Verbose(true),
		expect.CheckDuration(time.Millisecond*300),
	)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	s := &session{
		id:       xid.New().String(),
		prompt:   prompt,
		promptRe: promptRe,
		ptmx:     ptmx,
		expctr:   expctr,
		output:   buf,
		done:     make(chan struct{}),
		logger:   logger,
	}

	data, _, err := s.waitForReady(time.Second)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// Bash 5.1+ has this enabled by default and it makes it harder to parse the output.
	// We disable it here so that the output is consistent between old and new bash versions.
	_, exitCode, err := s.Execute("bind 'set enable-bracketed-paste off'", time.Second)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to disable bracketed paste")
	}
	if exitCode != 0 {
		return nil, nil, errors.Wrapf(err, "failed to disable bracketed paste: %d", exitCode)
	}

	// Reset buffer as we don't want to send the setting changes back.
	_, _ = io.Copy(io.Discard, s.output)

	go func() {
		s.err = <-cmdErr
		close(s.done)
	}()

	return s, data, nil
}

func (s *session) ID() string {
	return s.id
}

func (s *session) Close() error {
	if err := s.expctr.Close(); err != nil {
		return errors.WithStack(err)
	}
	<-s.done
	return s.err
}

func (s *session) Send(data []byte) error {
	return s.expctr.Send(string(data))
}

func (s *session) Read(p []byte) (int, error) {
	return s.output.Read(p)
}

func (s *session) setPrompt(prompt string) (err error) {
	s.prompt = prompt
	s.promptRe, err = compileLiteralRegexp(prompt)
	if err != nil {
		return errors.WithStack(err)
	}
	return
}

func (s *session) ChangePrompt(prompt string) (err error) {
	prevPrompt := s.prompt
	prevPromptRe := s.promptRe
	defer func() {
		if err != nil {
			s.prompt = prevPrompt
			s.promptRe = prevPromptRe
		}
	}()

	if err := s.setPrompt(prompt + "$"); err != nil {
		return err
	}

	// We don't use Execute() here because setting a prompt
	// makes matching it more difficult as different results
	// are possible.
	// To do it properly, we send the command and examine
	// the expected result. If the result contains
	// the new prompt twice, we don't need to do additional
	// check, which is required otherwise.
	err = s.expctr.Send(fmt.Sprintf("PS1='%s$ ' PS2='%s+ ' PROMPT_COMMAND=''\n", prompt, prompt))
	if err != nil {
		return errors.Wrap(err, "failed to send command")
	}

	data, match, err := s.expctr.Expect(s.promptRe, time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to wait for prompt")
	}

	switch strings.Count(data, match[0]) {
	case 0:
		return errors.New("unreachable")
	case 1:
		// Wait for the prompt again because above execute() call was matched by
		// the command itself.
		_, _, err = s.waitForReady(time.Second)
		return err
	default:
		return nil
	}
}

func (s *session) waitForReady(timeout time.Duration) ([]byte, []byte, error) {
	data, match, err := s.expctr.Expect(s.promptRe, timeout)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for prompt")
	}
	return []byte(data), []byte(match[0]), nil
}

func (s *session) Execute(command string, timeout time.Duration) ([]byte, int, error) {
	s.logger.Info("execute command", zap.String("command", command))

	if s.err != nil {
		return nil, -1, errors.WithStack(s.err)
	}

	data, err := s.execute(command, timeout)
	if err != nil {
		return nil, -1, err
	}

	exitCode, err := s.exitCode()
	return data, exitCode, err
}

func (s *session) ExecuteWithWriter(command string, timeout time.Duration, w io.Writer) (int, error) {
	s.logger.Info("execute command with writer", zap.String("command", command))

	if s.err != nil {
		return -1, errors.WithStack(s.err)
	}

	chunks := make(chan []byte)
	errC := make(chan error, 1)

	go func() {
		defer close(errC)
		for line := range chunks {
			_, err := w.Write(line)
			if err != nil {
				// Propagate only the first error.
				select {
				case errC <- errors.Wrap(err, "failed to write to writer"):
				default:
				}
			}
		}
	}()

	err := s.executeWithChannel(command, timeout, chunks)
	if err != nil {
		close(chunks)
		return -1, err
	}

	close(chunks)

	if err := <-errC; err != nil {
		return -1, err
	}

	return s.exitCode()
}

func (s *session) ExecuteWithChannel(command string, timeout time.Duration, chunks chan<- []byte) (int, error) {
	s.logger.Info("execute command with channel", zap.String("command", command))

	if s.err != nil {
		return -1, errors.WithStack(s.err)
	}

	err := s.executeWithChannel(command, timeout, chunks)
	if err != nil {
		return -1, err
	}

	return s.exitCode()
}

func (s *session) execute(command string, timeout time.Duration) ([]byte, error) {
	if command[len(command)-1] != '\n' {
		command += "\n"
	}

	if err := s.expctr.Send(command); err != nil {
		return nil, errors.Wrap(err, "failed to send command")
	}

	data, match, err := s.waitForReady(timeout)
	if err != nil {
		return nil, err
	}

	s.logger.Info("matched data", zap.ByteString("data", data))

	data = dropANSIEscape(data)

	commandLinesCount := strings.Count(command, "\n")
	for i := 0; i < commandLinesCount; i++ {
		idx := bytes.Index(data, crlf)
		if idx > -1 {
			data = data[idx+len(crlf):]
		} else {
			break
		}
	}

	data = bytes.TrimSpace(data)
	data = bytes.TrimSuffix(data, match)
	data = bytes.TrimSpace(data)

	s.logger.Info("cleaned matched data", zap.ByteString("data", data))

	return data, nil
}

type readCloser struct {
	io.Reader
	closed atomic.Bool
}

func (r *readCloser) Close() error {
	r.closed.Store(true)
	return nil
}

func (r *readCloser) Read(p []byte) (int, error) {
	if r.closed.Load() {
		return 0, io.EOF
	}
	return r.Reader.Read(p)
}

func (s *session) executeWithChannel(command string, timeout time.Duration, chunks chan<- []byte) error {
	if command[len(command)-1] != '\n' {
		command += "\n"
	}

	// TODO: s.output is shared here and in Read().
	// FIgure out a better solution.
	s.output.Reset()

	start := time.Now()

	if err := s.expctr.Send(command); err != nil {
		return errors.Wrap(err, "failed to send command")
	}

	// commandRe, err := compileLiteralRegexp(strings.Trim(command, "\r\n"))
	// if err != nil {
	// 	return -1, errors.Wrap(err, "failed to compile command to regexp")
	// }

	// _, _, err = s.expctr.Expect(commandRe, timeout-time.Since(start))
	// if err != nil {
	// 	return -1, errors.Wrap(err, "failed to wait for echoed command")
	// }

	errC := make(chan error, 1)
	reader := &readCloser{Reader: s.output}
	promptBytes := []byte(s.prompt)

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			return scanLinesUntil(data, atEOF, promptBytes)
		})

		for scanner.Scan() {
			line := scanner.Bytes()
			chunks <- line
		}

		errC <- scanner.Err()
	}()

	_, _, err := s.waitForReady(timeout)
	if err != nil {
		return err
	}

	_ = reader.Close()

	select {
	case err := <-errC:
		return err
	case <-time.After(timeout - time.Since(start)):
		return errors.New("waiting for read loop end timed out")
	}
}

const exitCodeCommand = "echo $?"

func (s *session) exitCode() (int, error) {
	data, err := s.execute(exitCodeCommand, time.Second)
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute exitCodeCommand")
	}
	code, err := strconv.ParseInt(string(data), 10, 16)
	if err != nil {
		return -1, errors.Wrap(err, "failed to parse exit code")
	}
	return int(code), nil
}

func spawnPty(command string, timeout time.Duration, opts ...expect.Option) (
	*exec.Cmd, *os.File, expect.Expecter, <-chan error, error,
) {
	cmd := exec.Command(command)
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	running := atomic.NewBool(true)
	resCh := make(chan error)

	expecter, cmdErr, err := expect.SpawnGeneric(&expect.GenOptions{
		In:  ptmx,
		Out: ptmx,
		Wait: func() error {
			return <-resCh
		},
		Close: func() error {
			close(resCh)
			running.Store(false)
			return cmd.Process.Kill()
		},
		Check: func() bool {
			return running.Load()
		},
	}, timeout, opts...)

	return cmd, ptmx, expecter, cmdErr, err
}

var crlf = []byte{'\r', '\n'}

// func dropCRPrefix(data []byte) []byte {
// 	if len(data) > 0 && data[0] == '\r' {
// 		return data[1:]
// 	}
// 	return data
// }

func compileLiteralRegexp(str string) (*regexp.Regexp, error) {
	b := new(strings.Builder)
	for _, c := range str {
		switch c {
		case '$', '^', '\\', '.', '|', '?', '*', '+', '(', ')', '[', ']', '{', '}':
			_, _ = b.WriteString(string([]rune{'\\', c}))
		default:
			_, _ = b.WriteRune(c)
		}
	}
	return regexp.Compile(b.String())
}

func scanLinesUntil(data []byte, atEOF bool, stop []byte) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	stopIdx := bytes.Index(data, stop)
	newLineIdx := bytes.IndexByte(data, '\n')
	if stopIdx >= 0 && (newLineIdx < 0 || stopIdx < newLineIdx) {
		return len(data), data[0:stopIdx], io.EOF
	} else if newLineIdx >= 0 {
		if len(data) > newLineIdx+1 && data[newLineIdx+1] == '\r' {
			newLineIdx++
		}
		return newLineIdx + 1, data[0 : newLineIdx+1], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
