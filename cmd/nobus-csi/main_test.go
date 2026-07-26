package main

import (
	"testing"

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
	if provider == nil {
		t.Fatalf("expected provider")
	}
}

func TestProviderFromConfig_MissingRealConfigReturnsError(t *testing.T) {
	_, err := providerFromConfig(driver.Config{})
	if err == nil {
		t.Fatalf("expected missing config error")
	}
}
