package server

import (
	"crypto/tls"
	"net"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/project/projectservice"
	"github.com/stateful/runme/v3/internal/runnerv2service"
	runmetls "github.com/stateful/runme/v3/internal/tls"
	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	projectv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/document/editor/editorservice"
)

const (
	maxMsgSize = 4 * 1024 * 1024 // 4 MiB
)

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

func New(
	c *Config,
	cmdFactory command.Factory,
	logger *zap.Logger,
) (_ *Server, err error) {
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
	runnerService, err := runnerv2service.NewRunnerService(cmdFactory, logger)
	if err != nil {
		return nil, err
	}
	runnerv2.RegisterRunnerServiceServer(grpcServer, runnerService)

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

func (s *Server) Addr() string {
	return s.lis.Addr().String()
}

func (s *Server) Serve() error {
	return s.grpcServer.Serve(s.lis)
}

func (s *Server) Shutdown() {
	s.grpcServer.GracefulStop()
}
