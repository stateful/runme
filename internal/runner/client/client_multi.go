package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/runner"
)

type MultiRunner struct {
	Runner Runner

	StdoutPrefix string

	PreRunMsg  func(blocks []*document.CodeBlock, parallel bool) string
	PostRunMsg func(block *document.CodeBlock, exitCode uint) string
}

type prefixWriter struct {
	w      io.Writer
	prefix string

	haveWritten bool
}

func NewPrefixWriter(w io.Writer, prefix string) io.Writer {
	return prefixWriter{
		w:      w,
		prefix: prefix,
	}
}

func (w prefixWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)

	// lines := make([][]byte, 0)
	// fmt.Println(p)

	// data := make([]byte, 0)

	// for _, line := range bytes.SplitAfter(p, []byte{'\n'}) {
	// 	newLine := make([]byte, 0)

	// 	// if !w.haveWritten || i != 0 {
	// 	// 	newLine = append(newLine, []byte(w.prefix)...)
	// 	// }

	// 	newLine = append(newLine, []byte("[pre] ")...)
	// 	newLine = append(newLine, line...)

	// 	// lines = append(lines, newLine)
	// 	data = append(data, newLine...)
	// }

	// w.haveWritten = true

	// return w.w.Write(data)
}

func (m MultiRunner) RunBlocks(ctx context.Context, blocks []*document.CodeBlock, parallel bool) error {
	if m.PreRunMsg != nil && parallel {
		m.Runner.getStdout().Write([]byte(
			m.PreRunMsg(blocks, parallel),
		))
	}

	errChan := make(chan error, len(blocks))
	var wg sync.WaitGroup

	for _, block := range blocks {
		runnerClient := m.Runner.Clone()

		if m.PreRunMsg != nil && !parallel {
			runnerClient.getStdout().Write([]byte(
				m.PreRunMsg([]*document.CodeBlock{block}, parallel),
			))
		}

		ApplyOptions(
			runnerClient,
			WithStdinTransform(func(r io.Reader) io.Reader {
				if parallel {
					// TODO: support stdin
					return bytes.NewReader(nil)
				}

				return r
			}),
			WithStdoutTransform(func(w io.Writer) io.Writer {
				if m.StdoutPrefix == "" {
					return w
				}

				prefix := fmt.Sprintf(
					m.StdoutPrefix,
					block.Name(),
				)

				return NewPrefixWriter(w, prefix)
			}),
		)

		run := func(block *document.CodeBlock) error {
			err := runnerClient.RunBlock(ctx, block)

			code := uint(0)

			{
				exitErr := (*runner.ExitError)(nil)
				if errors.As(err, &exitErr) {
					code = exitErr.Code
				}
			}

			if m.PostRunMsg != nil {
				runnerClient.getStdout().Write([]byte(
					m.PostRunMsg(block, code),
				))
			}

			return err
		}

		if !parallel {
			err := run(block)
			if err != nil {
				return err
			}
		} else {
			wg.Add(1)
			go func(block *document.CodeBlock) {
				defer wg.Done()
				err := run(block)
				errChan <- err
			}(block)
		}
	}

	wg.Wait()

	errors := make([]error, 0)

outer:
	for {
		select {
		case err := <-errChan:
			errors = append(errors, err)
		default:
			break outer
		}
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

func (m MultiRunner) Cleanup(ctx context.Context) error {
	return m.Runner.Cleanup(ctx)
}
