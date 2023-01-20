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
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"github.com/stateful/runme/expect"
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
	id        string
	prompt    string
	promptRe  *regexp.Regexp
	ptmx      *os.File
	expctr    expect.Expecter
	output    *limitedBuffer
	execGuard chan struct{}
	done      chan struct{}
	mx        sync.RWMutex
	err       error
	logger    *zap.Logger
}

func newSession(command, prompt string, logger *zap.Logger) (*session, []byte, error) {
	promptRe, err := compileLiteralRegexp(prompt)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	buf := &limitedBuffer{
		Buffer: bytes.NewBuffer(nil),
		state:  new(atomic.Uint32),
		more:   make(chan struct{}),
		closed: make(chan struct{}),
	}

	_, ptmx, expctr, cmdErr, err := spawnPty(
		command,
		-1,
		expect.Tee(buf),
		expect.Verbose(true),
		expect.CheckDuration(time.Millisecond*300),
		expect.PartialMatch(true),
	)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	s := &session{
		id:        xid.New().String(),
		prompt:    prompt,
		promptRe:  promptRe,
		ptmx:      ptmx,
		expctr:    expctr,
		output:    buf,
		execGuard: make(chan struct{}, 1),
		done:      make(chan struct{}),
		logger:    logger,
	}

	data, match, err := s.waitForReady(time.Second)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// Bash 5.1+ has this enabled by default and it makes it harder to parse the output.
	// We disable it here so that the output is consistent between old and new bash versions.
	//
	// Note that it has consequences. When bracketed paste is enabled, you can paste
	// a multi-line command and it won't be executed until you hit the Enter key:
	//
	//   bash-5.2$ echo 1
	//   echo 2
	//   1
	//   2
	//
	// If it's disabled, you will likely see the following result:
	//
	//   bash-5.2$ echo 1
	//   1
	//   bash-5.2$ echo 2
	//   2
	//
	// In our case, when keeping it disabled, the natural cause is that pasting
	// multiple commands in one request may result in undefined behaviour.
	// Source: https://askubuntu.com/a/1251430
	_, exitCode, err := s.Execute("bind 'set enable-bracketed-paste off'", time.Second)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to disable bracketed paste")
	}
	if exitCode != 0 {
		return nil, nil, errors.Wrapf(err, "failed to disable bracketed paste: %d", exitCode)
	}

	// Reset buffer as we don't want to send the setting changes back.
	_, _ = io.Copy(io.Discard, s.output)

	// Write the matched prompt back as invitation.
	_, _ = s.output.Write(match)
	_, _ = s.output.WriteRune(' ')

	go func() {
		s.setErr(<-cmdErr)
		close(s.done)
	}()

	return s, data, nil
}

func (s *session) getErr() error {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.err
}

func (s *session) setErr(err error) {
	if err == nil {
		return
	}
	s.mx.Lock()
	s.err = err
	s.mx.Unlock()
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

func (s *session) acquireWithTimeout(timeout *time.Duration) bool {
	start := time.Now()
	select {
	case s.execGuard <- struct{}{}:
		*timeout -= time.Since(start)
		return true
	case <-time.After(*timeout):
		return false
	}
}

func (s *session) free() {
	<-s.execGuard
}

func (s *session) setPrompt(prompt string) (err error) {
	s.prompt = prompt
	s.promptRe, err = compileLiteralRegexp(prompt)
	if err != nil {
		return errors.WithStack(err)
	}
	return
}

func (s *session) changePrompt(prompt string) (err error) {
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

var errBusy = errors.New("session is busy")

func (s *session) Execute(command string, timeout time.Duration) ([]byte, int, error) {
	s.logger.Info("execute command", zap.String("command", command))

	if err := s.getErr(); err != nil {
		return nil, -1, err
	}

	if ok := s.acquireWithTimeout(&timeout); !ok {
		return nil, -1, errBusy
	}
	defer s.free()

	data, err := s.execute(command, timeout)
	if err != nil {
		return nil, -1, err
	}

	exitCode, err := s.exitCode()
	return data, exitCode, err
}

func (s *session) ExecuteWithWriter(command string, timeout time.Duration, w io.Writer) (int, error) {
	s.logger.Info("execute command with writer", zap.String("command", command))

	if err := s.getErr(); err != nil {
		return -1, err
	}

	if ok := s.acquireWithTimeout(&timeout); !ok {
		return -1, errBusy
	}
	defer s.free()

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

	if err := s.getErr(); err != nil {
		return -1, err
	}

	if ok := s.acquireWithTimeout(&timeout); !ok {
		return -1, errBusy
	}
	defer s.free()

	err := s.executeWithChannel(command, timeout, chunks)
	if err != nil {
		return -1, err
	}

	return s.exitCode()
}

func ensureEndsWithNewLine(command string) string {
	if n := len(command); n > 0 && command[n-1] != '\n' {
		command += "\n"
	}
	return command
}

func cleanOutput(command string, data, match []byte) []byte {
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

	return data
}

func (s *session) execute(command string, timeout time.Duration) ([]byte, error) {
	command = ensureEndsWithNewLine(command)

	if err := s.expctr.Send(command); err != nil {
		return nil, errors.Wrap(err, "failed to send command")
	}

	data, match, err := s.waitForReady(timeout)
	if err != nil {
		return nil, err
	}

	s.logger.Info("matched data", zap.ByteString("data", data))

	data = cleanOutput(command, data, match)

	s.logger.Info("cleaned matched data", zap.ByteString("data", data))

	return data, nil
}

func (s *session) executeWithChannel(command string, timeout time.Duration, chunks chan<- []byte) error {
	command = ensureEndsWithNewLine(command)

	// TODO: s.output is shared here and in Read().
	// Figure out a better solution.
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
	defer func() { _ = reader.Close() }()

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			return scanLinesUntil(data, atEOF, []byte(s.prompt))
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

var exitCommandRe = func() *regexp.Regexp {
	re, err := compileLiteralRegexp(exitCodeCommand + "\r\n")
	if err != nil {
		panic(err)
	}
	return re
}()

func (s *session) exitCode() (int, error) {
	command := ensureEndsWithNewLine(exitCodeCommand)

	if err := s.expctr.Send(command); err != nil {
		return -1, errors.Wrap(err, "failed to send command")
	}

	_, _, err := s.expctr.Expect(exitCommandRe, time.Second)
	if err != nil {
		return -1, errors.Wrap(err, "failed to wait for command")
	}

	data, _, err := s.waitForReady(time.Second)
	if err != nil {
		return -1, err
	}

	s.logger.Info("matched data", zap.ByteString("data", data))

	lines := bytes.SplitN(data, crlf, 2)
	if len(lines) == 0 {
		return -1, errors.New("failed to locate exit code")
	}

	s.logger.Info("cleaned matched data", zap.ByteString("data", lines[0]))

	code, err := strconv.ParseInt(string(lines[0]), 10, 16)
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

	running := new(atomic.Bool)
	running.Store(true)
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
