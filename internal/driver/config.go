package driver

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

const (
	DefaultDriverName             = "csi.nobus.io"
	DefaultVersion                = "0.1.0"
	DefaultEndpoint               = "unix:///csi/csi.sock"
	DefaultMinimumVolumeBytes     = int64(1 << 30)
	DefaultVolumeGranularityBytes = int64(1 << 30)
)

type Mode string

const (
	ModeController Mode = "controller"
	ModeNode       Mode = "node"
	ModeAll        Mode = "all"
)

type Config struct {
	DriverName             string
	Version                string
	Endpoint               string
	Mode                   Mode
	APIURL                 string
	Token                  string
	Email                  string
	Password               string
	AllowFake              bool
	ProjectID              string
	AvailabilityZone       string
	VolumeType             string
	MinimumVolumeBytes     int64
	VolumeGranularityBytes int64
	OperationTimeout       time.Duration
	Logger                 *slog.Logger
}

func ConfigFromEnv() Config {
	endpoint := os.Getenv("CSI_ENDPOINT")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return Config{
		DriverName:             envString("NOBUS_DRIVER_NAME", DefaultDriverName),
		Version:                envString("NOBUS_DRIVER_VERSION", DefaultVersion),
		Endpoint:               endpoint,
		Mode:                   ModeAll,
		APIURL:                 os.Getenv("NOBUS_API_URL"),
		Token:                  os.Getenv("NOBUS_TOKEN"),
		Email:                  os.Getenv("NOBUS_EMAIL"),
		Password:               os.Getenv("NOBUS_PASSWORD"),
		AllowFake:              os.Getenv("NOBUS_ALLOW_FAKE") == "true",
		ProjectID:              os.Getenv("NOBUS_PROJECT_ID"),
		AvailabilityZone:       os.Getenv("NOBUS_AVAILABILITY_ZONE"),
		VolumeType:             os.Getenv("NOBUS_VOLUME_TYPE"),
		MinimumVolumeBytes:     DefaultMinimumVolumeBytes,
		VolumeGranularityBytes: DefaultVolumeGranularityBytes,
		OperationTimeout:       2 * time.Minute,
		Logger:                 slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (c Config) Validate() error {
	if c.DriverName == "" {
		return fmt.Errorf("driver name is required")
	}
	if c.Version == "" {
		return fmt.Errorf("driver version is required")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if c.Mode != ModeController && c.Mode != ModeNode && c.Mode != ModeAll {
		return fmt.Errorf("unsupported mode %q", c.Mode)
	}
	return nil
}

func envString(key string, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}
