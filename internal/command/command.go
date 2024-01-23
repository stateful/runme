package command

import (
	"io"

	"go.uber.org/zap"
)

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
