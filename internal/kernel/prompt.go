package kernel

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func DetectPrompt(cmdName string) ([]byte, error) {
	return detectPrompt(cmdName)
}

func detectPrompt(cmdName string) ([]byte, error) {
	var (
		output []byte
		err    error
	)

	switch {
	case strings.HasSuffix(cmdName, "bash"):
		output, err = exec.Command(cmdName, "-i", "-c", "echo ${PS1@P}").CombinedOutput()
	case strings.HasSuffix(cmdName, "zsh"):
		output, err = exec.Command(cmdName, "-i", "-c", "print -P $PS1").CombinedOutput()
	default:
		err = errors.New("unsupported shell")
	}
	if err == nil && len(output) == 0 {
		err = errors.New("empty prompt")
	}
	if err != nil {
		return nil, err
	}

	promptSlice := bytes.Split(output, []byte{'\n'})

	// Find the last non-empty line and consider this to be a prompt
	// we will be looking for.
	var prompt []byte
	for i := len(promptSlice) - 1; i >= 0; i-- {
		s := promptSlice[i]
		if len(s) > 0 {
			prompt = s
			break
		}
	}
	return prompt, nil
}
