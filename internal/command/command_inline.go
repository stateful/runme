package command

import (
	"bytes"
	"context"
)

type inlineCommand struct {
	internalCommand
}

var _ Command = (*inlineCommand)(nil)

func newInline(internal internalCommand) *inlineCommand {
	return &inlineCommand{
		internalCommand: internal,
	}
}

func (c *inlineCommand) Start(ctx context.Context) error {
	buf := new(bytes.Buffer)
	bw := bulkWriter{Writer: buf}
	cfg := c.ProgramConfig()

	// Write the script from the commands or the script.
	if commands := cfg.GetCommands(); commands != nil {
		for _, cmd := range commands.Items {
			bw.WriteString(cmd)
			bw.WriteByte('\n')
		}
	} else if script := cfg.GetScript(); script != "" {
		bw.WriteString(script)
	}

	// TODO(adamb): "-c" is not supported for all inline programs.
	if val := buf.String(); val != "" {
		cfg.Arguments = append(cfg.Arguments, val)
	}

	return c.internalCommand.Start(ctx)
}
