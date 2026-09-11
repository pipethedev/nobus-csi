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
				"instance-id": "server-2",
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

func TestReadPath_PrefersV1OverDS(t *testing.T) {
	path := writeMetadata(t, `{
		"v1": {
			"instance_id": "server-v1",
			"availability_zone": "az-v1",
			"region": "region-v1"
		},
		"ds": {
			"meta_data": {
				"instance-id": "server-ds",
				"availability_zone": "az-ds",
				"region": "region-ds"
			}
		}
	}`)
	instance, err := readPath(path)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if instance.ID != "server-v1" || instance.AvailabilityZone != "az-v1" || instance.Region != "region-v1" {
		t.Fatalf("expected v1 fields to win, got %+v", instance)
	}
}

func TestReadPath_MissingInstanceID_ReturnsInvalid(t *testing.T) {
	path := writeMetadata(t, `{"v1":{"availability_zone":"az1"}}`)
	_, err := readPath(path)
	if !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected os.ErrInvalid, got %v", err)
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
