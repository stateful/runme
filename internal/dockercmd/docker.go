package dockercmd

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

type Options struct {
	// BuildContext string
	// Dockerfile   string
	Image string

	Logger *zap.Logger
}

type Factory interface {
	CommandContext(context.Context, string, ...string) *Cmd
}

var _ Factory = (*docker)(nil)

func New(opts *Options) (Factory, error) {
	// Typically, the version is dicted by the Docker API version in the CI (GitHub Actions).
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.43"))
	if err != nil {
		return nil, err
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	rnd := rand.New(rand.NewSource(uint64(time.Now().Unix())))

	return &docker{
		Client: c,
		Image:  opts.Image,
		logger: logger,
		rnd:    rnd,
	}, nil
}

type docker struct {
	*client.Client
	Image string

	logger *zap.Logger
	rnd    *rand.Rand
}

func (d *docker) CommandContext(ctx context.Context, program string, args ...string) *Cmd {
	return &Cmd{
		Path: program,
		Args: args,

		ctx:    ctx,
		docker: d,
		name:   d.containerUniqueName(),

		logger: d.logger,
	}
}

func (d *docker) containerUniqueName() string {
	var hash [4]byte
	_, _ = d.rnd.Read(hash[:])
	return "runme-kernel-" + hex.EncodeToString(hash[:])
}
