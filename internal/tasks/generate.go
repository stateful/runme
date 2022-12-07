package tasks

import (
	"github.com/go-playground/validator/v10"
	"github.com/google/shlex"
	"github.com/pkg/errors"
)

func Generate(descriptions ...TaskDescription) (*TaskConfiguration, error) {
	tc := TaskConfiguration{
		Version: "2.0.0",
		BaseTaskConfiguration: BaseTaskConfiguration{
			Tasks: descriptions,
		},
	}
	v := validator.New()
	if err := v.Struct(tc); err != nil {
		return nil, errors.WithStack(err)
	}
	return &tc, nil
}

type ShellCommandOpts struct {
	Cwd string
	Env map[string]string
}

func GenerateFromShellCommand(name, command string, opts *ShellCommandOpts) (*TaskConfiguration, error) {
	fragments, err := shlex.Split(command)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse command %q", command)
	}

	var args []string
	if len(fragments) >= 1 {
		args = fragments[1:]
	}

	var options *CommandOptions
	if opts != nil {
		options = &CommandOptions{
			Cwd: opts.Cwd,
			Env: opts.Env,
		}
	}

	return Generate(TaskDescription{
		Label:   name,
		Type:    "shell",
		Command: fragments[0],
		Args:    args,
		Group:   "build",
		Options: options,
	})
}
