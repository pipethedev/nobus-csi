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

const testGiB = int64(1 << 30)

func TestCreateVolume_ExistingNameSameSpec_ReturnsExisting(t *testing.T) {
	driver := newTestDriver()
	req := createVolumeRequest(testGiB)
	first, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("create first volume: %v", err)
	}
	second, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("create existing volume: %v", err)
	}
	if second.GetVolume().GetVolumeId() != first.GetVolume().GetVolumeId() {
		t.Fatalf("expected existing volume id %q, got %q", first.GetVolume().GetVolumeId(), second.GetVolume().GetVolumeId())
	}
}

func TestCreateVolume_ExistingNameDifferentSize_ReturnsAlreadyExists(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create first volume: %v", err)
	}
	_, err = driver.CreateVolume(context.Background(), createVolumeRequest(2*testGiB))
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %s (%v)", status.Code(err), err)
	}
}

func TestCreateVolume_UnknownParameter_ReturnsInvalidArgument(t *testing.T) {
	driver := newTestDriver()
	req := createVolumeRequest(testGiB)
	req.Parameters["typo"] = "value"
	_, err := driver.CreateVolume(context.Background(), req)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s (%v)", status.Code(err), err)
	}
}

func TestValidateVolumeCapabilities_MultiNodeWriter_ReturnsMessage(t *testing.T) {
	driver := newTestDriver()
	resp, err := driver.ValidateVolumeCapabilities(context.Background(), &csi.ValidateVolumeCapabilitiesRequest{
		VolumeId: "vol-1",
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: mountAccess(),
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("validate capabilities: %v", err)
	}
	if resp.GetConfirmed() != nil {
		t.Fatalf("expected unconfirmed response")
	}
	if resp.GetMessage() == "" {
		t.Fatalf("expected rejection message")
	}
}

func TestControllerPublishVolume_AttachedElsewhere_ReturnsFailedPrecondition(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	_, err = driver.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		NodeId:   "server-a",
	})
	if err != nil {
		t.Fatalf("publish first node: %v", err)
	}
	_, err = driver.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		NodeId:   "server-b",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %s (%v)", status.Code(err), err)
	}
}

func TestControllerUnpublishVolume_MissingNode_ReturnsOK(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "vol-missing",
		NodeId:   "server-missing",
	})
	if err != nil {
		t.Fatalf("expected missing detach to be ok: %v", err)
	}
}

func TestControllerUnpublishVolume_EmptyNodeDetachesAll(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	_, err = driver.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		NodeId:   "server-a",
	})
	if err != nil {
		t.Fatalf("publish volume: %v", err)
	}
	_, err = driver.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("unpublish all: %v", err)
	}
	volume, err := driver.cloud.GetVolumeByID(context.Background(), created.GetVolume().GetVolumeId())
	if err != nil {
		t.Fatalf("get volume: %v", err)
	}
	if len(volume.Attachments) != 0 {
		t.Fatalf("expected no attachments, got %+v", volume.Attachments)
	}
}

func TestDeleteVolume_MissingVolume_ReturnsOK(t *testing.T) {
	driver := New(testConfig(), alwaysNotFoundCloud{}, mount.NewFake())
	_, err := driver.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: "missing"})
	if err != nil {
		t.Fatalf("expected missing delete to be ok: %v", err)
	}
}

func TestListVolumes_OpaqueStartingToken_ReturnsOK(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.ListVolumes(context.Background(), &csi.ListVolumesRequest{
		StartingToken: "opaque-token",
		MaxEntries:    10,
	})
	if err != nil {
		t.Fatalf("list volumes with opaque token: %v", err)
	}
}

func TestCreateVolume_TopologyMatchesNodeGetInfo(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	node, err := driver.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
	if err != nil {
		t.Fatalf("get node info: %v", err)
	}
	volumeSegments := created.GetVolume().GetAccessibleTopology()[0].GetSegments()
	nodeSegments := node.GetAccessibleTopology().GetSegments()
	for key, value := range volumeSegments {
		if nodeSegments[key] != value {
			t.Fatalf("expected node topology %s=%q, got %q", key, value, nodeSegments[key])
		}
	}
}

func TestCreateVolume_PreferredTopologyZoneIsIdempotent(t *testing.T) {
	driver := newTestDriver()
	req := createVolumeRequest(testGiB)
	req.AccessibilityRequirements = &csi.TopologyRequirement{
		Preferred: []*csi.Topology{
			{Segments: map[string]string{TopologyZoneKey: "az2"}},
		},
	}
	first, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	second, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("retry create volume: %v", err)
	}
	if first.GetVolume().GetVolumeId() != second.GetVolume().GetVolumeId() {
		t.Fatalf("expected idempotent volume id %q, got %q", first.GetVolume().GetVolumeId(), second.GetVolume().GetVolumeId())
	}
	zone := second.GetVolume().GetAccessibleTopology()[0].GetSegments()[TopologyZoneKey]
	if zone != "az2" {
		t.Fatalf("expected az2 topology, got %q", zone)
	}
}

func TestControllerUnpublishVolume_UsesProviderVolumeZone(t *testing.T) {
	driver := newTestDriver()
	req := createVolumeRequest(testGiB)
	req.Parameters[paramAvailabilityZone] = "az2"
	created, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	_, err = driver.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		NodeId:   "server-a",
	})
	if err != nil {
		t.Fatalf("publish volume: %v", err)
	}
	_, err = driver.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		NodeId:   "server-a",
	})
	if err != nil {
		t.Fatalf("unpublish volume: %v", err)
	}
	volume, err := driver.cloud.GetVolumeByID(context.Background(), created.GetVolume().GetVolumeId())
	if err != nil {
		t.Fatalf("get volume: %v", err)
	}
	if len(volume.Attachments) != 0 {
		t.Fatalf("expected no attachments, got %+v", volume.Attachments)
	}
}

func TestControllerExpandVolume_GrowsVolume(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	resp, err := driver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 2 * testGiB,
		},
	})
	if err != nil {
		t.Fatalf("expand volume: %v", err)
	}
	if resp.GetCapacityBytes() != 2*testGiB {
		t.Fatalf("expected expanded size %d, got %d", 2*testGiB, resp.GetCapacityBytes())
	}
	if !resp.GetNodeExpansionRequired() {
		t.Fatalf("expected node expansion to be required")
	}
}

func TestCreateSnapshot_SourceVolumeMissing_ReturnsNotFound(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: "missing",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %s (%v)", status.Code(err), err)
	}
}

func TestCreateSnapshot_SourceVolumeExists_ReturnsSnapshot(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if resp.GetSnapshot().GetSnapshotId() == "" {
		t.Fatalf("expected snapshot id")
	}
}

func newTestDriver() *Driver {
	config := testConfig()
	provider := cloud.NewFake(cloud.InstanceMetadata{
		InstanceID:        "server-a",
		AvailabilityZone:  "az1",
		Region:            "region",
		MaxVolumesPerNode: 32,
	})
	return New(config, provider, mount.NewFake())
}

func testConfig() Config {
	return Config{
		DriverName:             DefaultDriverName,
		Version:                DefaultVersion,
		Mode:                   ModeAll,
		Endpoint:               DefaultEndpoint,
		ProjectID:              "project",
		AvailabilityZone:       "az1",
		MinimumVolumeBytes:     testGiB,
		VolumeGranularityBytes: testGiB,
	}
}

func createVolumeRequest(sizeBytes int64) *csi.CreateVolumeRequest {
	return &csi.CreateVolumeRequest{
		Name: "data",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: sizeBytes,
		},
		Parameters: map[string]string{
			paramProjectID:        "project",
			paramAvailabilityZone: "az1",
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: mountAccess(),
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
	}
}

func mountAccess() *csi.VolumeCapability_Mount {
	return &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"},
	}
}

type alwaysNotFoundCloud struct {
	cloud.Cloud
}

func (alwaysNotFoundCloud) DeleteVolume(context.Context, string) error {
	return cloud.ErrNotFound
}
