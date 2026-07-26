package driver

import (
	"context"
	"errors"

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
	snapshot, err := d.cloud.CreateSnapshot(ctx, cloud.SnapshotSpec{
		Name:             req.GetName(),
		VolumeID:         req.GetSourceVolumeId(),
		ProjectID:        d.config.ProjectID,
		AvailabilityZone: firstValue(volume.AvailabilityZone, d.config.AvailabilityZone),
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
	if err := d.cloud.DeleteSnapshot(ctx, req.GetSnapshotId()); err != nil {
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

func csiSnapshot(snapshot cloud.Snapshot) *csi.Snapshot {
	return &csi.Snapshot{
		SizeBytes:      snapshot.SizeBytes,
		SnapshotId:     snapshot.ID,
		SourceVolumeId: snapshot.VolumeID,
		CreationTime:   timestamppb.Now(),
		ReadyToUse:     snapshot.Status == cloud.SnapshotStatusAvailable,
	}
}
