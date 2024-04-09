package dockerexec

import (
	"context"
	"encoding/hex"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

type Options struct {
	BuildContext string
	Dockerfile   string
	Image        string
	Logger       *zap.Logger
}

type Factory interface {
	CommandContext(context.Context, string, ...string) *Cmd
	LookPath(string) (string, error)
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

	d := &docker{
		Client: c,
		Image:  opts.Image,
		logger: logger,
		rnd:    rnd,
	}

	if err := d.buildOrPullImage(context.Background()); err != nil {
		return nil, err
	}

	return d, nil
}

type docker struct {
	*client.Client
	BuildContext string
	Dockerfile   string
	Image        string

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

func (d *docker) LookPath(path string) (string, error) {
	return "", errors.New("not implemented")
}

func (d *docker) containerUniqueName() string {
	var hash [4]byte
	_, _ = d.rnd.Read(hash[:])
	return "runme-kernel-" + hex.EncodeToString(hash[:])
}

func (d *docker) buildOrPullImage(ctx context.Context) error {
	if d.BuildContext != "" {
		return d.buildImage(ctx)
	}
	return d.pullImage(ctx)
}

func (d *docker) buildImage(ctx context.Context) error {
	return errors.New("not implemented")
}

func (d *docker) pullImage(ctx context.Context) error {
	filters := filters.NewArgs(filters.Arg("reference", d.Image))
	result, err := d.ImageList(ctx, types.ImageListOptions{Filters: filters})
	if err != nil {
		return errors.WithStack(err)
	}

	if len(result) > 0 {
		return nil
	}

	resp, err := d.ImagePull(ctx, d.Image, types.ImagePullOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp)
	return nil
}
