package runner

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/runmedev/runme/v3/internal/ansi"
	rcontext "github.com/runmedev/runme/v3/internal/runner/context"
	"github.com/runmedev/runme/v3/internal/ulid"
	"github.com/runmedev/runme/v3/pkg/project"
)

func (s *Session) loadDirEnv(ctx context.Context, proj *project.Project) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session is nil")
	}

	if proj == nil || !proj.EnvDirEnvEnabled() {
		return "", nil
	}

	preEnv, err := proj.LoadEnv()
	if err != nil {
		return "", err
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cfg := &ExecutableConfig{
		Dir:     proj.Root(),
		Name:    "LoadDirEnv",
		PreEnv:  preEnv,
		Session: s,
		Stderr:  stderr,
		Stdin:   bytes.NewBuffer([]byte("")),
		Stdout:  stdout,
		Tty:     false,
	}

	const sourceDirEnv = "which direnv && eval $(direnv export $SHELL)"
	exec := &Shell{
		ExecutableConfig: cfg,
		Cmds:             []string{sourceDirEnv},
	}

	const dirEnvRc = ".envrc"
	rctx := rcontext.WithExecutionInfo(ctx, &rcontext.ExecutionInfo{
		RunID:       ulid.GenerateID(),
		ExecContext: dirEnvRc,
	})

	if err = exec.Run(rctx); err != nil {
		// skip errors caused by clients creating a new session on delete running on shutdown
		if errors.Is(err, context.Canceled) {
			return err.Error(), nil
		}

		// this means direnv isn't installed == not an error
		if exec.ExitCode() > 0 && bytes.Contains(stdout.Bytes(), []byte("not found")) {
			return "direnv not found", nil
		}

		return "", err
	}

	msg := "unavailable"
	if stderr.Len() > 0 {
		msg = string(bytes.Trim(ansi.Strip(stderr.Bytes()), "\r\n"))
	}

	return msg, nil
}
