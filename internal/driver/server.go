package driver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/mount"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

type Server struct {
	config Config
	cloud  cloud.Cloud
	mount  mount.Interface
	server *grpc.Server
	log    *slog.Logger
}

func NewServer(config Config, provider cloud.Cloud, mounter mount.Interface) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	server := grpc.NewServer()
	driver := New(config, provider, mounter)
	csi.RegisterIdentityServer(server, driver)
	switch config.Mode {
	case ModeController:
		csi.RegisterControllerServer(server, driver)
	case ModeNode:
		csi.RegisterNodeServer(server, driver)
	case ModeAll:
		csi.RegisterControllerServer(server, driver)
		csi.RegisterNodeServer(server, driver)
	}
	return &Server{
		config: config,
		cloud:  provider,
		mount:  mounter,
		server: server,
		log:    config.Logger,
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	address, err := unixAddress(s.config.Endpoint)
	if err != nil {
		return err
	}
	s.log.Info("preparing csi socket", "endpoint", s.config.Endpoint, "socket", address)
	if err := os.RemoveAll(address); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	listener, err := net.Listen("unix", address)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	s.log.Info("csi grpc server listening", "mode", s.config.Mode, "socket", address)
	errs := make(chan error, 1)
	go func() {
		errs <- s.server.Serve(listener)
	}()
	select {
	case <-ctx.Done():
		s.log.Info("stopping csi grpc server")
		s.server.GracefulStop()
		s.log.Info("csi grpc server stopped")
		return nil
	case err := <-errs:
		return err
	}
}

func unixAddress(endpoint string) (string, error) {
	if strings.HasPrefix(endpoint, "tcp://") {
		return "", fmt.Errorf("tcp endpoint is unsupported")
	}
	if !strings.HasPrefix(endpoint, "unix://") {
		return "", fmt.Errorf("endpoint must use unix://")
	}
	address := strings.TrimPrefix(endpoint, "unix://")
	if address == "" {
		return "", fmt.Errorf("unix socket path is required")
	}
	return address, nil
}
