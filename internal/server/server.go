package server

import (
	"crypto/tls"
	"net"
	"os"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/config"
	runmetls "github.com/stateful/runme/v3/internal/tls"
	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	projectv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

type Server struct {
	gs     *grpc.Server
	lis    net.Listener
	logger *zap.Logger
}

func New(
	cfg *config.Config,
	parserService parserv1.ParserServiceServer,
	projectService projectv1.ProjectServiceServer,
	runnerService runnerv2.RunnerServiceServer,
	logger *zap.Logger,
) (*Server, error) {
	tlsCfg, err := createTLSConfig(cfg, logger)
	if err != nil {
		return nil, err
	}

	lis, err := createListener(cfg, tlsCfg)
	if err != nil {
		return nil, err
	}

	grpcServer := createGRPCServer(
		cfg,
		tlsCfg,
		parserService,
		projectService,
		runnerService,
	)

	s := Server{
		gs:     grpcServer,
		lis:    lis,
		logger: logger.Named("Server"),
	}

	return &s, nil
}

func (s *Server) Addr() string {
	return s.lis.Addr().String()
}

func (s *Server) Serve() error {
	s.logger.Info("starting gRPC server", zap.String("address", s.Addr()))
	return s.gs.Serve(s.lis)
}

func (s *Server) Shutdown() {
	s.logger.Info("stopping gRPC server")
	s.gs.GracefulStop()
}

func createTLSConfig(cfg *config.Config, logger *zap.Logger) (*tls.Config, error) {
	if tls := cfg.Server.Tls; tls != nil && tls.Enabled {
		// TODO(adamb): redesign runmetls API.
		return runmetls.LoadOrGenerateConfig(
			*tls.CertFile, // guaranteed in [getRootConfig]
			*tls.KeyFile,  // guaranteed in [getRootConfig]
			logger,
		)
	}
	return nil, nil
}

func createListener(cfg *config.Config, tlsCfg *tls.Config) (net.Listener, error) {
	addr := cfg.Server.Address
	protocol := "tcp"

	if strings.HasPrefix(addr, "unix://") {
		protocol = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
		if _, err := os.Stat(addr); !os.IsNotExist(err) {
			return nil, err
		}
	}

	if tlsCfg != nil {
		lis, err := tls.Listen(protocol, addr, tlsCfg)
		return lis, errors.WithStack(err)
	}

	lis, err := net.Listen(protocol, addr)
	return lis, errors.WithStack(err)
}

func createGRPCServer(
	cfg *config.Config,
	tlsCfg *tls.Config,
	parserService parserv1.ParserServiceServer,
	projectService projectv1.ProjectServiceServer,
	runnerService runnerv2.RunnerServiceServer,
) *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.Server.MaxMessageSize),
		grpc.MaxSendMsgSize(cfg.Server.MaxMessageSize),
		grpc.Creds(credentials.NewTLS(tlsCfg)),
	)

	// Register runme services.
	parserv1.RegisterParserServiceServer(grpcServer, parserService)
	projectv1.RegisterProjectServiceServer(grpcServer, projectService)
	runnerv2.RegisterRunnerServiceServer(grpcServer, runnerService)

	// Register health service.
	healthcheck := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthcheck)
	// Setting SERVING for the whole system.
	healthcheck.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	// Register reflection service.
	reflection.Register(grpcServer)

	return grpcServer
}
