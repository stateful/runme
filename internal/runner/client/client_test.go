package client

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stateful/runme/v3/internal/runner"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/pkg/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func TestResolveDirectory(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	root := filepath.Clean(
		filepath.Join(
			filepath.Dir(b),
			filepath.FromSlash("../../../"),
		),
	)

	projectRoot := filepath.Join(root, "examples/frontmatter/cwd")

	// repo path
	rp := func(rel string) string {
		return filepath.Join(root, filepath.FromSlash(rel))
	}

	proj, err := project.NewDirProject(projectRoot)
	require.NoError(t, err)

	tasks, err := project.LoadTasks(context.Background(), proj)
	require.NoError(t, err)

	taskMap := make(map[string]string, len(tasks))

	for _, task := range tasks {
		resolved := ResolveDirectory(root, task)
		taskMap[task.CodeBlock.Name()] = resolved
	}

	if runtime.GOOS == "windows" {
		assert.Equal(t, rp("examples\\frontmatter\\cwd"), taskMap["none-pwd"])
		assert.Equal(t, rp("examples\\frontmatter"), taskMap["none-rel-pwd"])

		assert.Equal(t, root, taskMap["relative-pwd"])
		assert.Equal(t, rp("../"), taskMap["relative-rel-pwd"])
	} else {
		assert.Equal(t, map[string]string{
			"absolute-pwd":     "/tmp",
			"absolute-rel-pwd": "/",
			"absolute-abs-pwd": "/opt",

			"none-pwd":     rp("examples/frontmatter/cwd"),
			"none-rel-pwd": rp("examples/frontmatter"),
			"none-abs-pwd": "/opt",

			"relative-pwd":     root,
			"relative-rel-pwd": rp("../"),
			"relative-abs-pwd": "/opt",
		}, taskMap)
	}
}

func testStartRunnerServiceServer(t *testing.T) (
	net.Listener,
	func(),
) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	lis, err := net.Listen("unix", fmt.Sprintf("/tmp/sock-%s.sock", ulid.Make()))
	require.NoError(t, err)
	server := grpc.NewServer()

	rss, err := runner.NewRunnerService(logger)
	require.NoError(t, err)

	runnerv1.RegisterRunnerServiceServer(server, rss)
	go server.Serve(lis)

	return lis, server.Stop
}

func TestResolveProgramLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	list, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)

	type Var struct {
		Status runnerv1.ResolveProgramResponse_Status
		Name   string
	}

	runnerOpts := []RunnerOption{
		WithInsecure(true),
	}
	localRunner, err := NewLocalRunner(runnerOpts...)
	assert.NoError(t, err)

	remoteRunner, err := NewRemoteRunner(
		context.Background(),
		fmt.Sprintf("unix://%s", list.Addr().String()),
		runnerOpts...,
	)
	assert.NoError(t, err)

	testCases := []struct {
		Title          string
		Mode           runnerv1.ResolveProgramRequest_Mode
		Input          string
		ExpectedScript string
		ExpectedVars   []Var
	}{
		{
			Title: "Mode UNSPECIFIED",
			Mode:  runnerv1.ResolveProgramRequest_MODE_UNSPECIFIED,
			Input: "$ export VARIABLE=Foo",
			ExpectedVars: []Var{
				{
					Status: runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE,
					Name:   "VARIABLE",
				},
			},
			ExpectedScript: `#
# VARIABLE set in managed env store
# "export VARIABLE=Foo"

`,
		},
		{
			Title: "Mode SKIP_ALL",
			Mode:  runnerv1.ResolveProgramRequest_MODE_SKIP_ALL,
			Input: "$ export VARIABLE=Foo",
			ExpectedVars: []Var{
				{
					Status: runnerv1.ResolveProgramResponse_STATUS_RESOLVED,
					Name:   "VARIABLE",
				},
			},
			ExpectedScript: `#
# VARIABLE set in managed env store
# "export VARIABLE=Foo"

`,
		},
		{
			Title: "Mode PROMPT_ALL",
			Mode:  runnerv1.ResolveProgramRequest_MODE_PROMPT_ALL,
			Input: "$ export VARIABLE=Foo",
			ExpectedVars: []Var{
				{
					Status: runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE,
					Name:   "VARIABLE",
				},
			},
			ExpectedScript: `#
# VARIABLE set in managed env store
# "export VARIABLE=Foo"

`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Title, func(t *testing.T) {
			for _, r := range []struct {
				name   string
				runner Runner
			}{
				{"local", localRunner},
				{"remote", remoteRunner},
			} {
				t.Run(r.name, func(t *testing.T) {
					ctx := context.Background()
					resp, err := r.runner.ResolveProgram(ctx, tt.Mode, tt.Input, "shell")
					require.NoError(t, err)

					var vars []Var
					for _, v := range resp.Vars {
						vars = append(vars, Var{
							Status: v.Status,
							Name:   v.Name,
						})
					}

					assert.NoError(t, err)
					assert.Equal(t, tt.ExpectedScript, resp.Script)
					assert.Equal(t, tt.ExpectedVars, vars)
				})
			}
		})
	}
}
