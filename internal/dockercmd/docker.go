package dockercmd

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/docker/docker/client"
	"golang.org/x/exp/rand"
)

type Options struct {
	// BuildContext string
	// Dockerfile   string
	Image string
}

type Factory interface {
	CommandContext(context.Context, string, ...string) *Cmd
}

var _ Factory = (*docker)(nil)

func New(opts *Options) (Factory, error) {
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	rnd := rand.New(rand.NewSource(uint64(time.Now().Unix())))

	return &docker{
		Client: c,
		Image:  opts.Image,
		rnd:    rnd,
	}, nil
}

type docker struct {
	*client.Client
	Image string

	rnd *rand.Rand
}

func (d *docker) CommandContext(ctx context.Context, program string, args ...string) *Cmd {
	return &Cmd{
		Path: program,
		Args: args,

		ctx:    ctx,
		docker: d,
		name:   d.containerUniqueName(),
	}
}

func (d *docker) containerUniqueName() string {
	var hash [4]byte
	_, _ = d.rnd.Read(hash[:])
	return "runme-kernel-" + hex.EncodeToString(hash[:])
}
