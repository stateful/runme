package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/browser"
)

type Env interface {
	IsAutonomous() bool
	RequestCode(url, state string) error
	WaitForCodeAndState(ctx context.Context) (string, string, error)
}

type desktopEnv struct {
	Session *oauthSession
}

var _ Env = (*desktopEnv)(nil)

func (desktopEnv) IsAutonomous() bool { return true }

func (e *desktopEnv) RequestCode(url, _ string) error {
	return browser.OpenURL(url)
}

func (e *desktopEnv) WaitForCodeAndState(ctx context.Context) (string, string, error) {
	return e.Session.WaitForCodeAndState(ctx)
}

type TerminalEnv struct {
	io.Reader // in
	io.Writer // out
}

var _ Env = (*TerminalEnv)(nil)

func (TerminalEnv) IsAutonomous() bool { return false }

func (e *TerminalEnv) RequestCode(url, state string) (err error) {
	_, err = fmt.Fprintf(e, "Open URL %s and wait for it to load ...\n", url)
	if err != nil {
		return
	}

	// There must be a new line character at the end,
	// otherwise Reader.ReadString('\n') won't work.
	_, err = fmt.Fprintf(
		e,
		"Validate the \"state\" query parameter is equal to %q and copy the value of the \"code\" query parameter value ... \n",
		state,
	)
	return
}

func (e *TerminalEnv) WaitForCodeAndState(context.Context) (string, string, error) {
	buf := bufio.NewReader(e)

	code, err := buf.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	code = strings.Trim(code, " \n")

	return code, "", nil
}

type TestEnv struct {
	Autonomous bool

	url   string
	state string
}

var _ Env = (*TestEnv)(nil)

func (e TestEnv) IsAutonomous() bool { return e.Autonomous }

func (e *TestEnv) RequestCode(url, state string) (err error) {
	e.url = url
	e.state = state
	return nil
}

func (e *TestEnv) WaitForCodeAndState(context.Context) (string, string, error) {
	return "test-code-1", e.state, nil
}
