package metadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPath_CloudInitV1_ReturnsInstance(t *testing.T) {
	path := writeMetadata(t, `{
		"v1": {
			"instance_id": "server-1",
			"availability_zone": "az1",
			"region": "region1"
		}
	}`)
	instance, err := readPath(path)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if instance.ID != "server-1" || instance.AvailabilityZone != "az1" || instance.Region != "region1" {
		t.Fatalf("unexpected instance: %+v", instance)
	}
}

func TestReadPath_ConfigDriveFallback_ReturnsInstance(t *testing.T) {
	path := writeMetadata(t, `{
		"ds": {
			"meta_data": {
				"instance_id": "server-2",
				"availability_zone": "az2",
				"region": "region2"
			}
		}
	}`)
	instance, err := readPath(path)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if instance.ID != "server-2" || instance.AvailabilityZone != "az2" || instance.Region != "region2" {
		t.Fatalf("unexpected instance: %+v", instance)
	}
}

func TestReadCloudInit_CanceledContext_ReturnsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReadCloudInit(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func writeMetadata(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "instance-data.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	return path
}
