//go:build !windows

package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/document/identity"
	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestVirtualCommand(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		cmd := NewVirtual(testConfigBasicProgram, Options{})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := NewVirtual(testConfigBasicProgram, Options{Stdout: stdout})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Getters", func(t *testing.T) {
		cmd := NewVirtual(
			&Config{
				ProgramName: "sleep",
				Arguments:   []string{"1"},
				Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{},
		)
		require.NoError(t, cmd.Start(context.Background()))

		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 1)
		require.NoError(t, cmd.Wait())
	})

	t.Run("SetWinsize", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)

		cmd := NewVirtual(
			&Config{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{"sleep 1", "tput cols -T linux", "tput lines -T linux"},
					},
				},
				Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{Stdout: stdout},
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, SetWinsize(cmd, &Winsize{Rows: 24, Cols: 80, X: 0, Y: 0}))
		require.NoError(t, cmd.Wait())
		require.Equal(t, "80\r\n24\r\n", stdout.String())
	})
}

func TestVirtualCommandFromCodeBlocksWithInputUsingPipe(t *testing.T) {
	idResolver := identity.NewResolver(identity.AllLifecycleIdentity)

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	t.Run("Cat", func(t *testing.T) {
		source := "```sh\ncat - | tr a-z A-Z\n```"
		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewVirtual(
			cfg,
			Options{
				Stdin:  stdinR,
				Stdout: stdout,
				Logger: logger,
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

		_, err = stdinW.Write([]byte("unit tests\n"))
		require.NoError(t, err)
		_, err = stdinW.Write([]byte{0x04}) // EOT
		require.NoError(t, err)

		require.NoError(t, command.Wait())
		require.Equal(t, "UNIT TESTS\r\n", stdout.String())
	})

	t.Run("Read", func(t *testing.T) {
		source := "```sh\nread name\necho \"My name is $name\"\n```"
		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewVirtual(
			cfg,
			Options{
				Stdin:  stdinR,
				Stdout: stdout,
				Logger: logger,
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

		_, err = stdinW.Write([]byte("Unit Test\n"))
		require.NoError(t, err)

		require.NoError(t, command.Wait())
		require.Equal(t, "My name is Unit Test\r\n", stdout.String())
	})

	t.Run("SimulateCtrlC", func(t *testing.T) {
		// Using sh start bash. We need to go deeper...
		source := "```sh\nbash\n```"

		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewVirtual(
			cfg,
			Options{
				Stdin:  stdinR,
				Stdout: stdout,
				Logger: logger,
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

		errc := make(chan error)
		go func() {
			defer close(errc)

			time.Sleep(time.Millisecond * 500)
			_, err = stdinW.Write([]byte("sleep 30\n"))
			errc <- err

			// cancel sleep
			time.Sleep(time.Millisecond * 500)
			_, err = stdinW.Write([]byte{3})
			errc <- err

			// terminate shell
			time.Sleep(time.Millisecond * 500)
			_, err = stdinW.Write([]byte{4})
			errc <- err

			// close writer; it's not needed
			errc <- stdinW.Close()
		}()
		for err := range errc {
			require.NoError(t, err)
		}

		require.EqualError(t, command.Wait(), "exit status 130")
	})
}
