package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/driver"
	"github.com/brimble/nobus-csi/internal/mount"
)

func main() {
	config := driver.ConfigFromEnv()
	flag.StringVar(&config.Endpoint, "endpoint", config.Endpoint, "CSI endpoint, unix:// only")
	flag.Func("mode", "controller, node, or all", func(value string) error {
		config.Mode = driver.Mode(value)
		return nil
	})
	flag.Parse()

	if err := run(config); err != nil {
		slog.Error("driver stopped", "error", err)
		os.Exit(1)
	}
}

func run(config driver.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config.Logger.Info("starting nobus csi driver",
		"driver", config.DriverName,
		"version", config.Version,
		"mode", config.Mode,
		"endpoint", config.Endpoint,
		"api_url_configured", config.APIURL != "",
		"token_configured", config.Token != "",
		"login_configured", config.Email != "" && config.Password != "",
		"project_configured", config.ProjectID != "",
		"availability_zone_configured", config.AvailabilityZone != "",
	)

	provider, err := providerFromConfig(config)
	if err != nil {
		return err
	}
	mounter := mount.New()
	server, err := driver.NewServer(config, provider, mounter)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return server.Serve(ctx)
}

func providerFromConfig(config driver.Config) (cloud.Cloud, error) {
	if config.APIURL != "" || config.Token != "" || config.Email != "" || config.Password != "" {
		return cloud.NewClient(cloud.ClientConfig{
			BaseURL:          config.APIURL,
			Token:            config.Token,
			Email:            config.Email,
			Password:         config.Password,
			ProjectID:        config.ProjectID,
			AvailabilityZone: config.AvailabilityZone,
		})
	}
	return cloud.NewFake(cloud.InstanceMetadata{
		InstanceID:        "nobus-test-instance",
		AvailabilityZone:  config.AvailabilityZone,
		Region:            "unknown",
		MaxVolumesPerNode: 32,
	}), nil
}
