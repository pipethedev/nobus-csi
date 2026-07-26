package driver

import (
	"context"
	"errors"
	"strings"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot name is required")
	}
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "source volume id is required")
	}
	volume, err := d.cloud.GetVolumeByID(ctx, req.GetSourceVolumeId())
	if err != nil {
		return nil, statusError(err)
	}
	zone := firstValue(volume.AvailabilityZone, d.config.AvailabilityZone)
	existing, err := d.snapshotByName(ctx, req.GetName(), zone)
	if err == nil {
		if existing.VolumeID != req.GetSourceVolumeId() {
			return nil, status.Error(codes.AlreadyExists, "snapshot name exists for a different source volume")
		}
		return &csi.CreateSnapshotResponse{Snapshot: csiSnapshot(*existing)}, nil
	}
	if !errors.Is(err, cloud.ErrNotFound) {
		return nil, statusError(err)
	}
	snapshot, err := d.cloud.CreateSnapshot(ctx, cloud.SnapshotSpec{
		Name:             req.GetName(),
		VolumeID:         req.GetSourceVolumeId(),
		ProjectID:        d.config.ProjectID,
		AvailabilityZone: zone,
		Force:            true,
		Metadata:         map[string]string{"csi.driver": d.config.DriverName},
	})
	if err != nil {
		return nil, statusError(err)
	}
	return &csi.CreateSnapshotResponse{Snapshot: csiSnapshot(*snapshot)}, nil
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot id is required")
	}
	id, zone := decodeSnapshotID(req.GetSnapshotId(), d.config.AvailabilityZone)
	if err := d.cloud.DeleteSnapshot(ctx, id, zone); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, statusError(err)
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	snapshots, token, err := d.cloud.ListSnapshots(ctx, cloud.Page{Token: req.GetStartingToken(), Size: int(req.GetMaxEntries())})
	if err != nil {
		return nil, statusError(err)
	}
	entries := make([]*csi.ListSnapshotsResponse_Entry, 0, len(snapshots))
	for _, snapshot := range snapshots {
		entries = append(entries, &csi.ListSnapshotsResponse_Entry{Snapshot: csiSnapshot(snapshot)})
	}
	return &csi.ListSnapshotsResponse{Entries: entries, NextToken: token}, nil
}

func (d *Driver) snapshotByName(ctx context.Context, name string, availabilityZone string) (*cloud.Snapshot, error) {
	snapshots, _, err := d.cloud.ListSnapshots(ctx, cloud.Page{Name: name, AvailabilityZone: availabilityZone})
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot.Name == name {
			clone := snapshot
			return &clone, nil
		}
	}
	return nil, cloud.ErrNotFound
}

func csiSnapshot(snapshot cloud.Snapshot) *csi.Snapshot {
	return &csi.Snapshot{
		SizeBytes:      snapshot.SizeBytes,
		SnapshotId:     encodeSnapshotID(snapshot.ID, snapshot.AvailabilityZone),
		SourceVolumeId: snapshot.VolumeID,
		CreationTime:   timestamppb.Now(),
		ReadyToUse:     snapshot.Status == cloud.SnapshotStatusAvailable,
	}
}

func encodeSnapshotID(id string, availabilityZone string) string {
	if availabilityZone == "" {
		return id
	}
	return availabilityZone + "/" + id
}

func decodeSnapshotID(id string, fallbackZone string) (string, string) {
	zone, snapshot, ok := strings.Cut(id, "/")
	if !ok {
		return id, fallbackZone
	}
	return snapshot, zone
}
