package driver

import (
	"context"
	"testing"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/mount"
	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestNodeGetInfo_EmptyRegion_OmitsRegionTopology(t *testing.T) {
	config := Config{
		DriverName:             DefaultDriverName,
		Version:                DefaultVersion,
		Mode:                   ModeAll,
		Endpoint:               DefaultEndpoint,
		MinimumVolumeBytes:     testGiB,
		VolumeGranularityBytes: testGiB,
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
	if segments["topology.csi.nobus.io/zone"] != "nova" {
		t.Fatalf("expected nova zone, got %q", segments["topology.csi.nobus.io/zone"])
	}
	if _, ok := segments["topology.csi.nobus.io/region"]; ok {
		t.Fatalf("expected empty region to be omitted")
	}
}
