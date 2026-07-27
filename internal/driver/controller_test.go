package driver

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

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

func TestControllerPublishVolume_MultiattachVolume_ReturnsFailedPrecondition(t *testing.T) {
	driver := New(testConfig(), multiattachCloud{}, mount.NewFake())
	_, err := driver.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: "vol-1",
		NodeId:   "server-a",
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

func TestListVolumes_StartingTokenFromPreviousPage_ReturnsOK(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create first volume: %v", err)
	}
	second := createVolumeRequest(testGiB)
	second.Name = "other-data"
	_, err = driver.CreateVolume(context.Background(), second)
	if err != nil {
		t.Fatalf("create second volume: %v", err)
	}
	first, err := driver.ListVolumes(context.Background(), &csi.ListVolumesRequest{
		MaxEntries: 1,
	})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if first.GetNextToken() == "" {
		t.Fatalf("expected next token")
	}
	_, err = driver.ListVolumes(context.Background(), &csi.ListVolumesRequest{
		StartingToken: first.GetNextToken(),
		MaxEntries:    1,
	})
	if err != nil {
		t.Fatalf("list next page: %v", err)
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

func TestCreateVolume_RequisiteTopologyPrefersFallbackWhenAllowed(t *testing.T) {
	driver := newTestDriver()
	req := createVolumeRequest(testGiB)
	req.AccessibilityRequirements = &csi.TopologyRequirement{
		Requisite: []*csi.Topology{
			{Segments: map[string]string{TopologyZoneKey: "az2"}},
			{Segments: map[string]string{TopologyZoneKey: "az1"}},
		},
	}
	created, err := driver.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	zone := created.GetVolume().GetAccessibleTopology()[0].GetSegments()[TopologyZoneKey]
	if zone != "az1" {
		t.Fatalf("expected fallback az1 topology, got %q", zone)
	}
}

func TestCreateVolume_DualControllerDuplicateCreate_ReturnsCanonicalAndDeletesOrphan(t *testing.T) {
	provider := newDualCreateCloud()
	first := New(testConfig(), provider, mount.NewFake())
	second := New(testConfig(), provider, mount.NewFake())
	req := createVolumeRequest(testGiB)
	responses := make([]*csi.CreateVolumeResponse, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		responses[0], errs[0] = first.CreateVolume(context.Background(), req)
	}()
	go func() {
		defer wg.Done()
		responses[1], errs[1] = second.CreateVolume(context.Background(), req)
	}()
	provider.waitForCreates(2)
	provider.releaseCreates()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("create volume: %v", err)
		}
	}
	if responses[0].GetVolume().GetVolumeId() != responses[1].GetVolume().GetVolumeId() {
		t.Fatalf("expected canonical id from both controllers, got %q and %q", responses[0].GetVolume().GetVolumeId(), responses[1].GetVolume().GetVolumeId())
	}
	if !provider.deleted("vol-2") {
		t.Fatalf("expected duplicate volume vol-2 to be deleted")
	}
}

func TestCreateVolume_StaggeredDuplicateVisibility_ReturnsStableCanonical(t *testing.T) {
	provider := newStaggeredCreateCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if resp.GetVolume().GetVolumeId() != "vol-1" {
		t.Fatalf("expected stable canonical vol-1, got %q", resp.GetVolume().GetVolumeId())
	}
	if !provider.deleted("vol-2") {
		t.Fatalf("expected staggered duplicate vol-2 to be deleted")
	}
}

func TestCreateVolume_PostCreateEmptyList_RetriesUntilVisible(t *testing.T) {
	provider := newLaggedCreateCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if resp.GetVolume().GetVolumeId() != "vol-1" {
		t.Fatalf("expected visible volume vol-1, got %q", resp.GetVolume().GetVolumeId())
	}
}

func TestCreateVolume_ReconciledVolumeNotReadable_ReturnsUnavailable(t *testing.T) {
	driver := New(testConfig(), unreadableCreatedVolumeCloud{}, mount.NewFake())
	_, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %s (%v)", status.Code(err), err)
	}
}

func TestCreateVolume_ReconcileDeletesAllCompatibleOrphans(t *testing.T) {
	provider := newOrphanCreateCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if resp.GetVolume().GetVolumeId() != "vol-1" {
		t.Fatalf("expected canonical volume vol-1, got %q", resp.GetVolume().GetVolumeId())
	}
	for _, id := range []string{"vol-2", "vol-3"} {
		if !provider.deleted(id) {
			t.Fatalf("expected duplicate volume %s to be deleted", id)
		}
	}
}

func TestCreateVolume_ReconcileSkipsAttachedDuplicate(t *testing.T) {
	provider := newAttachedOrphanCreateCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if resp.GetVolume().GetVolumeId() != "vol-1" {
		t.Fatalf("expected canonical volume vol-1, got %q", resp.GetVolume().GetVolumeId())
	}
	if provider.deleted("vol-2") {
		t.Fatalf("expected attached duplicate vol-2 not to be deleted")
	}
	if !provider.deleted("vol-3") {
		t.Fatalf("expected available duplicate vol-3 to be deleted")
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

func TestControllerExpandVolume_AlreadyLargeEnough_ReturnsExistingSize(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(2*testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	resp, err := driver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: created.GetVolume().GetVolumeId(),
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: testGiB,
		},
	})
	if err != nil {
		t.Fatalf("expand volume: %v", err)
	}
	if resp.GetCapacityBytes() != 2*testGiB {
		t.Fatalf("expected existing size %d, got %d", 2*testGiB, resp.GetCapacityBytes())
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

func TestCreateSnapshot_ExistingNameSameSource_ReturnsExisting(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	first, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	second, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create existing snapshot: %v", err)
	}
	if first.GetSnapshot().GetSnapshotId() != second.GetSnapshot().GetSnapshotId() {
		t.Fatalf("expected existing snapshot id %q, got %q", first.GetSnapshot().GetSnapshotId(), second.GetSnapshot().GetSnapshotId())
	}
}

func TestCreateSnapshot_ExistingNameSameSource_ReturnsStableCreationTime(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	first, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	second, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create existing snapshot: %v", err)
	}
	if !first.GetSnapshot().GetCreationTime().AsTime().Equal(second.GetSnapshot().GetCreationTime().AsTime()) {
		t.Fatalf("expected stable creation time")
	}
}

func TestCSISnapshot_UsesDomainCreationTime(t *testing.T) {
	created := time.Unix(1700000000, 0).UTC()
	snapshot := csiSnapshot(cloud.Snapshot{
		ID:               "snap-1",
		VolumeID:         "vol-1",
		SizeBytes:        testGiB,
		Status:           cloud.SnapshotStatusAvailable,
		CreatedAt:        created,
		AvailabilityZone: "az1",
	})
	if !snapshot.GetCreationTime().AsTime().Equal(created) {
		t.Fatalf("expected provider creation time %s, got %s", created, snapshot.GetCreationTime().AsTime())
	}
}

func TestCreateSnapshot_ExistingNameSameSourceDeletesOrphan(t *testing.T) {
	provider := newSnapshotOrphanCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: "vol-1",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if resp.GetSnapshot().GetSnapshotId() != "az1/snap-1" {
		t.Fatalf("expected canonical snapshot az1/snap-1, got %q", resp.GetSnapshot().GetSnapshotId())
	}
	if !provider.deleted("snap-2") {
		t.Fatalf("expected duplicate snapshot snap-2 to be deleted")
	}
}

func TestCreateSnapshot_ExistingNameSameSourceToleratesCleanupConflict(t *testing.T) {
	provider := newSnapshotConflictOrphanCloud()
	driver := New(testConfig(), provider, mount.NewFake())
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: "vol-1",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if resp.GetSnapshot().GetSnapshotId() != "az1/snap-1" {
		t.Fatalf("expected canonical snapshot az1/snap-1, got %q", resp.GetSnapshot().GetSnapshotId())
	}
	if !provider.deleted("snap-2") {
		t.Fatalf("expected duplicate snapshot delete to be attempted")
	}
}

func TestCreateSnapshot_ExistingNameDifferentSource_ReturnsAlreadyExists(t *testing.T) {
	driver := newTestDriver()
	first, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create first volume: %v", err)
	}
	secondReq := createVolumeRequest(testGiB)
	secondReq.Name = "other-data"
	second, err := driver.CreateVolume(context.Background(), secondReq)
	if err != nil {
		t.Fatalf("create second volume: %v", err)
	}
	_, err = driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: first.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	_, err = driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: second.GetVolume().GetVolumeId(),
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %s (%v)", status.Code(err), err)
	}
}

func TestDeleteSnapshot_EncodedZoneDeletesWithSnapshotZone(t *testing.T) {
	driver := newTestDriver()
	created, err := driver.CreateVolume(context.Background(), createVolumeRequest(testGiB))
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	snapshot, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: created.GetVolume().GetVolumeId(),
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	_, err = driver.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{
		SnapshotId: snapshot.GetSnapshot().GetSnapshotId(),
	})
	if err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
}

func TestCreateSnapshot_EncodesRequestZoneWhenProviderOmitsZone(t *testing.T) {
	driver := New(testConfig(), newSnapshotNoAZCloud(), mount.NewFake())
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: "vol-1",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if resp.GetSnapshot().GetSnapshotId() != "az2/snap-1" {
		t.Fatalf("expected zone-encoded snapshot id, got %q", resp.GetSnapshot().GetSnapshotId())
	}
}

func TestCreateSnapshot_PostCreateEmptyList_RetriesUntilVisible(t *testing.T) {
	driver := New(testConfig(), newLaggedSnapshotCloud(), mount.NewFake())
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "backup",
		SourceVolumeId: "vol-1",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if resp.GetSnapshot().GetSnapshotId() != "az1/snap-1" {
		t.Fatalf("expected visible snapshot az1/snap-1, got %q", resp.GetSnapshot().GetSnapshotId())
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

type multiattachCloud struct {
	cloud.Cloud
}

func (multiattachCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
		Multiattach:      true,
	}, nil
}

type dualCreateCloud struct {
	cloud.Cloud
	mu         sync.Mutex
	volumes    map[string]cloud.Volume
	deletedIDs []string
	created    chan struct{}
	released   chan struct{}
}

func newDualCreateCloud() *dualCreateCloud {
	return &dualCreateCloud{
		volumes:  make(map[string]cloud.Volume),
		created:  make(chan struct{}, 2),
		released: make(chan struct{}),
	}
}

func (d *dualCreateCloud) GetVolumeByName(context.Context, string, string) (*cloud.Volume, error) {
	return nil, cloud.ErrNotFound
}

func (d *dualCreateCloud) CreateVolume(_ context.Context, spec cloud.VolumeSpec) (*cloud.Volume, error) {
	d.mu.Lock()
	id := "vol-1"
	if len(d.volumes) > 0 {
		id = "vol-2"
	}
	volume := cloud.Volume{
		ID:               id,
		Name:             spec.Name,
		SizeBytes:        spec.SizeBytes,
		AvailabilityZone: spec.AvailabilityZone,
		Type:             spec.Type,
	}
	d.volumes[id] = volume
	d.mu.Unlock()
	d.created <- struct{}{}
	<-d.released
	return &volume, nil
}

func (d *dualCreateCloud) ListVolumes(_ context.Context, page cloud.Page) ([]cloud.Volume, string, error) {
	select {
	case <-d.released:
	default:
		return nil, "", nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	volumes := make([]cloud.Volume, 0, len(d.volumes))
	for _, volume := range d.volumes {
		if page.Name != "" && volume.Name != page.Name {
			continue
		}
		if page.AvailabilityZone != "" && volume.AvailabilityZone != page.AvailabilityZone {
			continue
		}
		volumes = append(volumes, volume)
	}
	return volumes, "", nil
}

func (d *dualCreateCloud) GetVolumeByID(_ context.Context, id string) (*cloud.Volume, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	volume, ok := d.volumes[id]
	if !ok {
		return nil, cloud.ErrNotFound
	}
	return &volume, nil
}

func (d *dualCreateCloud) DeleteVolume(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deletedIDs = append(d.deletedIDs, id)
	delete(d.volumes, id)
	return nil
}

func (d *dualCreateCloud) waitForCreates(count int) {
	for range count {
		<-d.created
	}
}

func (d *dualCreateCloud) releaseCreates() {
	close(d.released)
}

func (d *dualCreateCloud) deleted(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Contains(d.deletedIDs, id)
}

type staggeredCreateCloud struct {
	cloud.Cloud
	mu         sync.Mutex
	listCalls  int
	deletedIDs []string
}

func newStaggeredCreateCloud() *staggeredCreateCloud {
	return &staggeredCreateCloud{}
}

func (s *staggeredCreateCloud) ListVolumes(context.Context, cloud.Page) ([]cloud.Volume, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	created := cloud.Volume{
		ID:               "vol-2",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}
	canonical := cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}
	if s.listCalls == 1 {
		return nil, "", nil
	}
	if s.listCalls == 2 {
		return []cloud.Volume{created}, "", nil
	}
	return []cloud.Volume{created, canonical}, "", nil
}

func (s *staggeredCreateCloud) CreateVolume(context.Context, cloud.VolumeSpec) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-2",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (s *staggeredCreateCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (s *staggeredCreateCloud) DeleteVolume(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedIDs = append(s.deletedIDs, id)
	return nil
}

func (s *staggeredCreateCloud) deleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.deletedIDs, id)
}

type snapshotNoAZCloud struct {
	cloud.Cloud
	created bool
}

func newSnapshotNoAZCloud() *snapshotNoAZCloud {
	return &snapshotNoAZCloud{}
}

func (s *snapshotNoAZCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az2",
	}, nil
}

func (s *snapshotNoAZCloud) ListSnapshots(context.Context, cloud.Page) ([]cloud.Snapshot, string, error) {
	if !s.created {
		return nil, "", nil
	}
	return []cloud.Snapshot{
		{ID: "snap-1", Name: "backup", VolumeID: "vol-1", SizeBytes: testGiB, Status: cloud.SnapshotStatusAvailable},
	}, "", nil
}

func (s *snapshotNoAZCloud) CreateSnapshot(context.Context, cloud.SnapshotSpec) (*cloud.Snapshot, error) {
	s.created = true
	return &cloud.Snapshot{
		ID:        "snap-1",
		Name:      "backup",
		VolumeID:  "vol-1",
		SizeBytes: testGiB,
		Status:    cloud.SnapshotStatusAvailable,
	}, nil
}

type laggedCreateCloud struct {
	cloud.Cloud
	mu        sync.Mutex
	created   bool
	listCalls int
}

func newLaggedCreateCloud() *laggedCreateCloud {
	return &laggedCreateCloud{}
}

func (l *laggedCreateCloud) ListVolumes(context.Context, cloud.Page) ([]cloud.Volume, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listCalls++
	if !l.created || l.listCalls <= 2 {
		return nil, "", nil
	}
	return []cloud.Volume{
		{
			ID:               "vol-1",
			Name:             "data",
			SizeBytes:        testGiB,
			AvailabilityZone: "az1",
		},
	}, "", nil
}

func (l *laggedCreateCloud) CreateVolume(context.Context, cloud.VolumeSpec) (*cloud.Volume, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.created = true
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (l *laggedCreateCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

type unreadableCreatedVolumeCloud struct {
	cloud.Cloud
}

func (unreadableCreatedVolumeCloud) CreateVolume(context.Context, cloud.VolumeSpec) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (unreadableCreatedVolumeCloud) ListVolumes(context.Context, cloud.Page) ([]cloud.Volume, string, error) {
	return []cloud.Volume{
		{
			ID:               "vol-1",
			Name:             "data",
			SizeBytes:        testGiB,
			AvailabilityZone: "az1",
		},
	}, "", nil
}

func (unreadableCreatedVolumeCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return nil, cloud.ErrNotFound
}

type orphanCreateCloud struct {
	cloud.Cloud
	mu         sync.Mutex
	created    bool
	deletedIDs []string
}

func newOrphanCreateCloud() *orphanCreateCloud {
	return &orphanCreateCloud{}
}

func (o *orphanCreateCloud) ListVolumes(context.Context, cloud.Page) ([]cloud.Volume, string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.created {
		return nil, "", nil
	}
	return []cloud.Volume{
		{ID: "vol-3", Name: "data", SizeBytes: testGiB, AvailabilityZone: "az1"},
		{ID: "vol-1", Name: "data", SizeBytes: testGiB, AvailabilityZone: "az1"},
		{ID: "vol-2", Name: "data", SizeBytes: testGiB, AvailabilityZone: "az1"},
	}, "", nil
}

func (o *orphanCreateCloud) CreateVolume(context.Context, cloud.VolumeSpec) (*cloud.Volume, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.created = true
	return &cloud.Volume{
		ID:               "vol-3",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (o *orphanCreateCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (o *orphanCreateCloud) DeleteVolume(_ context.Context, id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.deletedIDs = append(o.deletedIDs, id)
	return nil
}

func (o *orphanCreateCloud) deleted(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Contains(o.deletedIDs, id)
}

type attachedOrphanCreateCloud struct {
	cloud.Cloud
	deletedIDs []string
}

func newAttachedOrphanCreateCloud() *attachedOrphanCreateCloud {
	return &attachedOrphanCreateCloud{}
}

func (a *attachedOrphanCreateCloud) ListVolumes(context.Context, cloud.Page) ([]cloud.Volume, string, error) {
	return []cloud.Volume{
		{ID: "vol-1", Name: "data", SizeBytes: testGiB, AvailabilityZone: "az1"},
		{
			ID:               "vol-2",
			Name:             "data",
			SizeBytes:        testGiB,
			Status:           cloud.VolumeStatusInUse,
			AvailabilityZone: "az1",
			Attachments:      []cloud.Attachment{{InstanceID: "server-b", DevicePath: "/dev/vdb"}},
		},
		{ID: "vol-3", Name: "data", SizeBytes: testGiB, AvailabilityZone: "az1"},
	}, "", nil
}

func (a *attachedOrphanCreateCloud) CreateVolume(context.Context, cloud.VolumeSpec) (*cloud.Volume, error) {
	return nil, cloud.ErrAlreadyExists
}

func (a *attachedOrphanCreateCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (a *attachedOrphanCreateCloud) DeleteVolume(_ context.Context, id string) error {
	a.deletedIDs = append(a.deletedIDs, id)
	return nil
}

func (a *attachedOrphanCreateCloud) deleted(id string) bool {
	return slices.Contains(a.deletedIDs, id)
}

type laggedSnapshotCloud struct {
	cloud.Cloud
	mu        sync.Mutex
	created   bool
	listCalls int
}

func newLaggedSnapshotCloud() *laggedSnapshotCloud {
	return &laggedSnapshotCloud{}
}

func (l *laggedSnapshotCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (l *laggedSnapshotCloud) ListSnapshots(context.Context, cloud.Page) ([]cloud.Snapshot, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listCalls++
	if !l.created || l.listCalls <= 2 {
		return nil, "", nil
	}
	return []cloud.Snapshot{
		{
			ID:               "snap-1",
			Name:             "backup",
			VolumeID:         "vol-1",
			SizeBytes:        testGiB,
			Status:           cloud.SnapshotStatusAvailable,
			AvailabilityZone: "az1",
		},
	}, "", nil
}

func (l *laggedSnapshotCloud) CreateSnapshot(context.Context, cloud.SnapshotSpec) (*cloud.Snapshot, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.created = true
	return &cloud.Snapshot{
		ID:               "snap-1",
		Name:             "backup",
		VolumeID:         "vol-1",
		SizeBytes:        testGiB,
		Status:           cloud.SnapshotStatusAvailable,
		AvailabilityZone: "az1",
	}, nil
}

type snapshotOrphanCloud struct {
	cloud.Cloud
	deletedIDs []string
}

func newSnapshotOrphanCloud() *snapshotOrphanCloud {
	return &snapshotOrphanCloud{}
}

func (s *snapshotOrphanCloud) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	return &cloud.Volume{
		ID:               "vol-1",
		Name:             "data",
		SizeBytes:        testGiB,
		AvailabilityZone: "az1",
	}, nil
}

func (s *snapshotOrphanCloud) ListSnapshots(context.Context, cloud.Page) ([]cloud.Snapshot, string, error) {
	return []cloud.Snapshot{
		{
			ID:               "snap-2",
			Name:             "backup",
			VolumeID:         "vol-1",
			SizeBytes:        testGiB,
			Status:           cloud.SnapshotStatusAvailable,
			AvailabilityZone: "az1",
		},
		{
			ID:               "snap-1",
			Name:             "backup",
			VolumeID:         "vol-1",
			SizeBytes:        testGiB,
			Status:           cloud.SnapshotStatusAvailable,
			AvailabilityZone: "az1",
		},
	}, "", nil
}

func (s *snapshotOrphanCloud) DeleteSnapshot(_ context.Context, id string, _ string) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return nil
}

func (s *snapshotOrphanCloud) deleted(id string) bool {
	return slices.Contains(s.deletedIDs, id)
}

type snapshotConflictOrphanCloud struct {
	snapshotOrphanCloud
}

func newSnapshotConflictOrphanCloud() *snapshotConflictOrphanCloud {
	return &snapshotConflictOrphanCloud{}
}

func (s *snapshotConflictOrphanCloud) DeleteSnapshot(_ context.Context, id string, _ string) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return cloud.ErrConflict
}
