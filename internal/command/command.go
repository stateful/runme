package command

import (
	"context"
	"io"
	"os"

	"go.uber.org/zap"
)

type Command interface {
	Pid() int
	Running() bool
	Start(context.Context) error
	Signal(os.Signal) error
	Wait() error
}

type Options struct {
	Kernel  Kernel
	Logger  *zap.Logger
	Session *Session
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type Factory func(*Config, Options) Command
