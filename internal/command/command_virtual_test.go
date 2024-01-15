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

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/identity"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestVirtualCommand1(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	stdout := bytes.NewBuffer(nil)
	opts := &VirtualCommandOptions{
		Stdout: stdout,
		Logger: logger,
	}
	cmd, err := NewVirtual(testConfigBasicProgram, opts)
	require.NoError(t, err)
	require.NoError(t, cmd.Start(context.Background()))
	require.NoError(t, cmd.Wait())
	assert.Equal(t, "test", stdout.String())
}

func TestVirtualCommand(t *testing.T) {
	t.Run("OptionsIsNil", func(t *testing.T) {
		cmd, err := NewVirtual(testConfigBasicProgram, nil)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		opts := &VirtualCommandOptions{
			Stdout: stdout,
		}
		cmd, err := NewVirtual(testConfigBasicProgram, opts)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Getters", func(t *testing.T) {
		cmd, err := NewVirtual(&Config{
			ProgramName: "sleep",
			Arguments:   []string{"1"},
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}, nil)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.True(t, cmd.IsRunning())
		require.Greater(t, cmd.PID(), 1)
		require.NoError(t, cmd.Wait())
	})

	t.Run("SetWinsize", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)

		cmd, err := NewVirtual(
			&Config{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{"sleep 1", "tput cols -T linux", "tput lines -T linux"},
					},
				},
				Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			&VirtualCommandOptions{Stdout: stdout},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.SetWinsize(24, 80, 0, 0))
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

		remoteOptions := &VirtualCommandOptions{
			Stdin:  stdinR,
			Stdout: stdout,
			Logger: logger,
		}
		command, err := NewVirtual(cfg, remoteOptions)
		require.NoError(t, err)

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

		remoteOptions := &VirtualCommandOptions{
			Stdin:  stdinR,
			Stdout: stdout,
			Logger: logger,
		}
		command, err := NewVirtual(cfg, remoteOptions)
		require.NoError(t, err)

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

		remoteOptions := &VirtualCommandOptions{
			Stdin:  stdinR,
			Stdout: stdout,
			Logger: logger,
		}
		command, err := NewVirtual(cfg, remoteOptions)
		require.NoError(t, err)

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
