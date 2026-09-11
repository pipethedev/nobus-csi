package mount

import (
	"context"
	"slices"
	"testing"
)

func TestFake_StagePublishUnmount(t *testing.T) {
	mounter := NewFake()
	ctx := context.Background()
	if err := mounter.Stage(ctx, "/dev/sda", "/staging", "ext4", nil); err != nil {
		t.Fatalf("stage: %v", err)
	}
	mounted, err := mounter.IsMounted(ctx, "/staging")
	if err != nil {
		t.Fatalf("inspect staging: %v", err)
	}
	if !mounted {
		t.Fatalf("expected staging path to be mounted")
	}
	if err := mounter.Publish(ctx, "/staging", "/target", false, false, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := mounter.Unmount(ctx, "/target"); err != nil {
		t.Fatalf("unmount target: %v", err)
	}
	if err := mounter.Unmount(ctx, "/staging"); err != nil {
		t.Fatalf("unmount staging: %v", err)
	}
	mounted, err = mounter.IsMounted(ctx, "/staging")
	if err != nil {
		t.Fatalf("inspect unmounted staging: %v", err)
	}
	if mounted {
		t.Fatalf("expected staging path to be unmounted")
	}
}

func TestFake_FindDevice_UsesHint(t *testing.T) {
	path, err := NewFake().FindDevice(context.Background(), "vol-1", "/dev/vdb")
	if err != nil {
		t.Fatalf("find device: %v", err)
	}
	if path != "/dev/vdb" {
		t.Fatalf("expected hinted path, got %q", path)
	}
}

func TestDeviceCandidates_IncludesQEMUNames(t *testing.T) {
	got := deviceCandidates("vol-1", "/dev/vdb")
	want := []string{
		"/dev/vdb",
		"/dev/disk/by-id/virtio-vol-1",
		"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_vol-1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
