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
	provider, err := providerFromConfig(config)
	if err != nil {
		return err
	}
	server, err := driver.NewServer(config, provider, mount.NewFake())
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return server.Serve(ctx)
}

func providerFromConfig(config driver.Config) (cloud.Cloud, error) {
	if config.APIURL != "" || config.Token != "" {
		return cloud.NewClient(cloud.ClientConfig{
			BaseURL:          config.APIURL,
			Token:            config.Token,
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
