package dockercmd

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type Cmd struct {
	Path   string
	Args   []string
	Env    []string
	Dir    string
	TTY    bool
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	ctx context.Context

	docker *docker

	containerID string
	name        string

	errC chan error
}

func (c *Cmd) Start() error {
	containerConfig := &container.Config{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          c.TTY,
		OpenStdin:    true,
		StdinOnce:    true,
		Env:          c.Env,
		Entrypoint:   []string{c.Path},
		Cmd:          c.Args,
		Image:        c.docker.Image,
		WorkingDir:   "/workspace",
	}
	hostConfig := &container.HostConfig{
		RestartPolicy:  container.RestartPolicy{Name: container.RestartPolicyDisabled},
		AutoRemove:     false,
		ConsoleSize:    [2]uint{80, 24},
		ReadonlyRootfs: true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: c.Dir,
				Target: "/workspace",
			},
		},
	}
	networkConfig := &network.NetworkingConfig{}
	platformConfig := &v1.Platform{}

	resp, err := c.docker.ContainerCreate(
		c.ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		platformConfig,
		c.name,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	c.containerID = resp.ID

	hijack, err := c.docker.ContainerAttach(
		c.ctx,
		c.containerID,
		container.AttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		},
	)
	if err != nil {
		return errors.WithStack(err)
	}

	err = c.docker.ContainerStart(
		c.ctx,
		resp.ID,
		container.StartOptions{},
	)
	if err != nil {
		return errors.WithStack(err)
	}

	c.errC = make(chan error, 2)

	go func() {
		var err error
		if c.TTY {
			_, err = io.Copy(c.Stdout, hijack.Reader)
		} else {
			_, err = stdcopy.StdCopy(c.Stdout, c.Stderr, hijack.Reader)
		}
		c.errC <- errors.WithStack(err)
	}()

	go func() {
		if c.Stdin == nil {
			return
		}
		_, err := io.Copy(hijack.Conn, c.Stdin)
		c.errC <- errors.WithStack(err)
	}()

	return nil
}

func (c *Cmd) Wait() error {
	_, waitC := c.docker.ContainerWait(c.ctx, c.containerID, container.WaitConditionNotRunning)

	select {
	case err := <-waitC:
		return errors.WithStack(err)
	case err := <-c.errC:
		return err
	}
}
