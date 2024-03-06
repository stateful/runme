package server

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stateful/runme/v3/internal/document/editor/editorservice"
	parserv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/parser/v1"
	projectv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/project/v1"
	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/v3/internal/project/projectservice"
	"github.com/stateful/runme/v3/internal/runnerv2service"
	runmetls "github.com/stateful/runme/v3/internal/tls"
)

const (
	maxMsgSize  = 4 * 1024 * 1024 // 4 MiB
	tlsFileMode = os.FileMode(0o700)
)

// TODO(adamb): this should not be here...
var defaultTLSDir = filepath.Join(getDefaultConfigHome(), "tls")

func GetDefaultTLSDir() string {
	return defaultTLSDir
}

type Config struct {
	Address    string
	CertFile   string
	KeyFile    string
	TLSEnabled bool
}

type Server struct {
	grpcServer *grpc.Server
	lis        net.Listener
	logger     *zap.Logger
}

func New(c *Config, logger *zap.Logger) (_ *Server, err error) {
	var tlsConfig *tls.Config

	if c.TLSEnabled {
		// TODO(adamb): redesign runmetls API.
		tlsConfig, err = runmetls.LoadOrGenerateConfig(c.CertFile, c.KeyFile, logger)
		if err != nil {
			return nil, err
		}
	}

	addr := c.Address
	protocol := "tcp"

	var lis net.Listener

	if strings.HasPrefix(addr, "unix://") {
		protocol = "unix"
		addr = strings.TrimPrefix(addr, "unix://")

		if _, err := os.Stat(addr); !os.IsNotExist(err) {
			return nil, err
		}
	}

	if tlsConfig == nil {
		lis, err = net.Listen(protocol, addr)
	} else {
		lis, err = tls.Listen(protocol, addr, tlsConfig)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}

	logger.Info("server listening", zap.String("address", addr))

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)

	// Register runme services.
	parserv1.RegisterParserServiceServer(grpcServer, editorservice.NewParserServiceServer(logger))
	projectv1.RegisterProjectServiceServer(grpcServer, projectservice.NewProjectServiceServer(logger))
	runnerService, err := runnerv2service.NewRunnerService(logger)
	if err != nil {
		return nil, err
	}
	runnerv2alpha1.RegisterRunnerServiceServer(grpcServer, runnerService)

	// Register health service.
	healthcheck := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthcheck)
	// Setting SERVING for the whole system.
	healthcheck.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	// Register reflection service.
	reflection.Register(grpcServer)

	return &Server{
		lis:        lis,
		grpcServer: grpcServer,
		logger:     logger,
	}, nil
}

func (s *Server) Serve() error {
	return s.grpcServer.Serve(s.lis)
}

func (s *Server) Shutdown() error {
	s.grpcServer.GracefulStop()
	return errors.WithStack(s.lis.Close())
}

func getDefaultConfigHome() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	_, fErr := os.Stat(dir)
	if os.IsNotExist(fErr) {
		mkdErr := os.MkdirAll(dir, 0o700)
		if mkdErr != nil {
			dir = os.TempDir()
		}
	}
	return filepath.Join(dir, "runme")
}
