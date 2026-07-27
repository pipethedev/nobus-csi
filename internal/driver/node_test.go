package driver

import (
	"context"
	"testing"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/mount"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodeGetInfo_EmptyRegion_OmitsRegionTopology(t *testing.T) {
	config := Config{
		DriverName:             DefaultDriverName,
		Version:                DefaultVersion,
		Mode:                   ModeAll,
		Endpoint:               DefaultEndpoint,
		MinimumVolumeBytes:     testGiB,
		VolumeGranularityBytes: testGiB,
		AvailabilityZone:       "nobus-wa-az2",
	}
	provider := cloud.NewFake(cloud.InstanceMetadata{
		InstanceID:       "server-1",
		AvailabilityZone: "nova",
	})
	driver := New(config, provider, mount.NewFake())
	resp, err := driver.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
	if err != nil {
		t.Fatalf("get node info: %v", err)
	}
	if resp.GetNodeId() != "server-1" {
		t.Fatalf("expected node id server-1, got %q", resp.GetNodeId())
	}
	segments := resp.GetAccessibleTopology().GetSegments()
	if segments["topology.csi.nobus.io/zone"] != "nobus-wa-az2" {
		t.Fatalf("expected configured zone, got %q", segments["topology.csi.nobus.io/zone"])
	}
	if _, ok := segments["topology.csi.nobus.io/region"]; ok {
		t.Fatalf("expected empty region to be omitted")
	}
}

func TestNodeGetInfo_ProviderZoneIsKeptWhenSpecific(t *testing.T) {
	config := testConfig()
	config.AvailabilityZone = "az1"
	provider := cloud.NewFake(cloud.InstanceMetadata{
		InstanceID:       "server-1",
		AvailabilityZone: "az2",
	})
	driver := New(config, provider, mount.NewFake())
	resp, err := driver.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
	if err != nil {
		t.Fatalf("get node info: %v", err)
	}
	segments := resp.GetAccessibleTopology().GetSegments()
	if segments[TopologyZoneKey] != "az2" {
		t.Fatalf("expected provider zone, got %q", segments[TopologyZoneKey])
	}
}

func TestNodeUnstageVolume_MissingVolumeID_ReturnsInvalidArgument(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{
		StagingTargetPath: "/staging",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s (%v)", status.Code(err), err)
	}
}

func TestNodeUnpublishVolume_MissingVolumeID_ReturnsInvalidArgument(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
		TargetPath: "/target",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s (%v)", status.Code(err), err)
	}
}

func TestNodePublishVolume_BlockModeResolvesDevice(t *testing.T) {
	mounter := mount.NewFake()
	driver := New(testConfig(), cloud.NewFake(cloud.InstanceMetadata{InstanceID: "server-1"}), mounter)
	_, err := driver.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:   "vol-1",
		TargetPath: "/target/block",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	})
	if err != nil {
		t.Fatalf("publish block volume: %v", err)
	}
	mounted, err := mounter.IsMounted(context.Background(), "/target/block")
	if err != nil {
		t.Fatalf("inspect fake mount: %v", err)
	}
	if !mounted {
		t.Fatalf("expected block target to be mounted")
	}
}
