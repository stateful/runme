package dockerexec

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

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

func (d *Docker) RemoveImage(ctx context.Context) error {
	_, err := d.client.ImageRemove(ctx, d.image, image.RemoveOptions{Force: true, PruneChildren: true})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
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

func (d *Docker) buildImage(ctx context.Context) error {
	tar, err := archive.TarWithOptions(d.buildContext, &archive.TarOptions{})
	if err != nil {
		return errors.WithMessage(err, "failed to create tar archive")
	}

	resp, err := d.client.ImageBuild(
		ctx,
		tar,
		types.ImageBuildOptions{
			Dockerfile:  d.dockerfile,
			Tags:        []string{d.image},
			Remove:      true,
			ForceRemove: true,
			NoCache:     true,
		},
	)
	if err != nil {
		return errors.WithMessage(err, "failed to build image")
	}
	defer resp.Body.Close()

	return errors.WithStack(
		logClientMessages(resp.Body, d.logger),
	)
}

func logClientMessages(r io.Reader, logger *zap.Logger) error {
	type errorLine struct {
		Error       string `json:"error"`
		ErrorDetail struct {
			Message string `json:"message"`
		} `json:"errorDetail"`
	}

	var lastLine string

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		lastLine = scanner.Text()
		logger.Debug("docker build", zap.String("log", lastLine))
	}

	if err := scanner.Err(); err != nil {
		return errors.WithMessage(err, "docker build")
	}

	errLine := errorLine{}
	if err := json.Unmarshal([]byte(lastLine), &errLine); err != nil {
		return errors.WithMessage(err, "docker build")
	}
	if errLine.Error != "" {
		return errors.Errorf("docker build: %s", errLine.Error)
	}
	return nil
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
