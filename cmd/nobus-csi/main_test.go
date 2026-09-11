package main

import (
	"runtime"
	"testing"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/driver"
)

func TestProviderFromConfig_AllowFakeUsesFakeProvider(t *testing.T) {
	provider, err := providerFromConfig(driver.Config{
		AllowFake:        true,
		AvailabilityZone: "az1",
	})
	if err != nil {
		t.Fatalf("create fake provider: %v", err)
	}
	if _, ok := provider.(*cloud.Fake); !ok {
		t.Fatalf("expected *cloud.Fake, got %T", provider)
	}
}

func TestProviderFromConfig_MissingRealConfigReturnsError(t *testing.T) {
	_, err := providerFromConfig(driver.Config{})
	if err == nil {
		t.Fatalf("expected missing config error")
	}
}

func TestRun_InvalidMode_ReturnsError(t *testing.T) {
	err := run(driver.Config{
		DriverName: driver.DefaultDriverName,
		Version:    driver.DefaultVersion,
		Endpoint:   driver.DefaultEndpoint,
		Mode:       "bogus",
	})
	if err == nil {
		t.Fatalf("expected invalid mode error")
	}
}

func TestRequireLinuxNode_RejectsNonLinuxWithoutFake(t *testing.T) {
	err := requireLinuxNode(driver.Config{Mode: driver.ModeNode})
	if runtime.GOOS == "linux" {
		if err != nil {
			t.Fatalf("linux node plugin should be allowed: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected non-linux node plugin without fake to fail")
	}
	if err := requireLinuxNode(driver.Config{Mode: driver.ModeAll, AllowFake: true}); err != nil {
		t.Fatalf("fake node plugin should be allowed: %v", err)
	}
	if err := requireLinuxNode(driver.Config{Mode: driver.ModeController}); err != nil {
		t.Fatalf("controller mode should be allowed: %v", err)
	}
}
