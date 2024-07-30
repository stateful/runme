package dockerexec

import (
	"context"
	"encoding/hex"
	"io"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

type Options struct {
	BuildContext string
	Debug        bool
	Dockerfile   string
	Image        string
	Logger       *zap.Logger
}

func New(opts *Options) (*Docker, error) {
	// Typically, the version is dicted by the Docker API version in the CI (GitHub Actions).
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.43"))
	if err != nil {
		return nil, err
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	rnd := rand.New(rand.NewSource(uint64(time.Now().UnixNano())))

	d := &Docker{
		client:       c,
		buildContext: opts.BuildContext,
		debug:        opts.Debug,
		dockerfile:   opts.Dockerfile,
		image:        opts.Image,
		logger:       logger,
		rnd:          rnd,
	}

	if err := d.buildOrPullImage(context.Background()); err != nil {
		return nil, err
	}

	return d, nil
}

type Docker struct {
	client       *client.Client
	buildContext string // used to build the image
	debug        bool   // when true, the container will not be removed
	dockerfile   string // used to build the image
	image        string

	logger *zap.Logger
	rnd    *rand.Rand
}

func (d *Docker) CommandContext(ctx context.Context, program string, args ...string) *Cmd {
	return &Cmd{
		Path: program,
		Args: args,

		ctx:    ctx,
		docker: d,
		name:   d.containerUniqueName(),

		logger: d.logger,
	}
}

func (d *Docker) containerUniqueName() string {
	var hash [4]byte
	_, _ = d.rnd.Read(hash[:])
	return "runme-runner-" + hex.EncodeToString(hash[:])
}

func (d *Docker) buildOrPullImage(ctx context.Context) error {
	if d.buildContext != "" {
		return d.buildImage(ctx)
	}
	return d.pullImage(ctx)
}

func (d *Docker) buildImage(context.Context) error {
	return errors.New("not implemented")
}

func (d *Docker) pullImage(ctx context.Context) error {
	filters := filters.NewArgs(filters.Arg("reference", d.image))
	result, err := d.client.ImageList(ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return errors.WithStack(err)
	}

	if len(result) > 0 {
		return nil
	}

	resp, err := d.client.ImagePull(ctx, d.image, image.PullOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp)
	return nil
}
