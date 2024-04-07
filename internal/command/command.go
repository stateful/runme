package command

import (
	"errors"
	"io"

	"github.com/stateful/runme/v3/internal/dockercmd"
	"go.uber.org/zap"
)

type DockerCommandOptions struct {
	CmdFactory dockercmd.Factory
	Session    *Session

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Logger *zap.Logger
}

func NewDocker(cfg *Config, options *DockerCommandOptions) (*DockerCommand, error) {
	if options == nil {
		return nil, errors.New("options cannot be nil")
	}

	if options.CmdFactory == nil {
		return nil, errors.New("CmdFactory cannot be nil")
	}

	if options.Logger == nil {
		options.Logger = zap.NewNop()
	}

	return newDockerCommand(cfg, options), nil
}

type NativeCommandOptions struct {
	Session *Session

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Logger *zap.Logger
}

func NewNative(cfg *Config, options *NativeCommandOptions) (*NativeCommand, error) {
	if options == nil {
		options = &NativeCommandOptions{}
	}

	if options.Logger == nil {
		options.Logger = zap.NewNop()
	}

	return newNativeCommand(cfg, options), nil
}

type VirtualCommandOptions struct {
	Session *Session

	Stdin  io.Reader
	Stdout io.Writer

	Logger *zap.Logger
}

func NewVirtual(cfg *Config, options *VirtualCommandOptions) (*VirtualCommand, error) {
	if options == nil {
		options = &VirtualCommandOptions{}
	}

	if options.Logger == nil {
		options.Logger = zap.NewNop()
	}

	return newVirtualCommand(cfg, options), nil
}
