package driver

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	createReconcileAttempts      = 7
	createReconcileInitialDelay  = 100 * time.Millisecond
	createReconcileStableSamples = 3
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := validateCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, err
	}
	spec, err := volumeSpec(req, d.config)
	if err != nil {
		return nil, err
	}
	unlock := d.names.TryLock(req.GetName())
	if unlock == nil {
		return nil, status.Error(codes.Aborted, "another create operation is in flight for this volume name")
	}
	defer unlock()
	existing, err := d.reconcileVolumeByName(ctx, spec)
	if err == nil {
		return &csi.CreateVolumeResponse{Volume: csiVolume(*existing)}, nil
	}
	if !errors.Is(err, cloud.ErrNotFound) {
		return nil, grpcError(err)
	}
	_, err = d.cloud.CreateVolume(ctx, spec)
	if err != nil {
		if errors.Is(err, cloud.ErrAlreadyExists) {
			reconciled, getErr := d.reconcileVolumeByName(ctx, spec)
			if getErr == nil {
				return &csi.CreateVolumeResponse{Volume: csiVolume(*reconciled)}, nil
			}
		}
		return nil, statusError(err)
	}
	reconciled, err := d.reconcileCreatedVolume(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &csi.CreateVolumeResponse{Volume: csiVolume(*reconciled)}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if err := d.cloud.DeleteVolume(ctx, req.GetVolumeId()); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, statusError(err)
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is required")
	}
	unlock := d.locks.TryLock(req.GetVolumeId())
	if unlock == nil {
		return nil, status.Error(codes.Aborted, "another operation is in flight for this volume")
	}
	defer unlock()
	volume, err := d.cloud.GetVolumeByID(ctx, req.GetVolumeId())
	zone := volumeContextValue(req.GetVolumeContext(), paramAvailabilityZone, d.config.AvailabilityZone)
	if err == nil {
		if volume.AvailabilityZone != "" {
			zone = volume.AvailabilityZone
		}
		if volume.Multiattach {
			return nil, statusError(cloud.ErrConflict)
		}
		device, attachedElsewhere := attachedDevice(*volume, req.GetNodeId())
		if device != "" {
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{"device_path": device},
			}, nil
		}
		if attachedElsewhere {
			return nil, statusError(cloud.ErrConflict)
		}
	} else if errors.Is(err, cloud.ErrNotFound) {
		return nil, statusError(err)
	} else {
		return nil, statusError(err)
	}
	device, err := d.cloud.AttachVolume(ctx, req.GetVolumeId(), req.GetNodeId(), zone)
	if err != nil {
		return nil, statusError(err)
	}
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{"device_path": device},
	}, nil
}

func attachedDevice(volume cloud.Volume, node string) (string, bool) {
	for _, attachment := range volume.Attachments {
		if attachment.InstanceID == node {
			return attachment.DevicePath, false
		}
	}
	return "", len(volume.Attachments) > 0
}

func compatibleVolume(volume cloud.Volume, spec cloud.VolumeSpec) bool {
	return volume.SizeBytes == spec.SizeBytes && volume.AvailabilityZone == spec.AvailabilityZone && volume.Type == spec.Type
}

func (d *Driver) reconcileCreatedVolume(ctx context.Context, spec cloud.VolumeSpec) (*cloud.Volume, error) {
	var previous string
	stable := 0
	delay := createReconcileInitialDelay
	for range createReconcileAttempts {
		canonical, err := d.reconcileVolumeByName(ctx, spec)
		if err != nil {
			if !errors.Is(err, cloud.ErrNotFound) {
				return nil, grpcError(err)
			}
			if err := wait(ctx, delay); err != nil {
				return nil, statusError(err)
			}
			delay *= 2
			continue
		}
		if canonical.ID == previous {
			stable++
		} else {
			previous = canonical.ID
			stable = 1
		}
		if stable >= createReconcileStableSamples {
			return canonical, nil
		}
		if err := wait(ctx, delay); err != nil {
			return nil, statusError(err)
		}
		delay *= 2
	}
	canonical, err := d.reconcileVolumeByName(ctx, spec)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, statusError(cloud.ErrUnavailable)
		}
		return nil, grpcError(err)
	}
	return canonical, nil
}

func (d *Driver) reconcileVolumeByName(ctx context.Context, spec cloud.VolumeSpec) (*cloud.Volume, error) {
	volumes, _, err := d.cloud.ListVolumes(ctx, cloud.Page{Name: spec.Name, AvailabilityZone: spec.AvailabilityZone})
	if err != nil {
		return nil, err
	}
	compatible := make([]cloud.Volume, 0, len(volumes))
	for _, volume := range volumes {
		if volume.Name != spec.Name {
			continue
		}
		if !compatibleVolume(volume, spec) {
			return nil, status.Error(codes.AlreadyExists, "volume name exists with different parameters")
		}
		compatible = append(compatible, volume)
	}
	if len(compatible) == 0 {
		return nil, cloud.ErrNotFound
	}
	slices.SortFunc(compatible, func(left cloud.Volume, right cloud.Volume) int {
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})
	canonical := compatible[0]
	for _, duplicate := range compatible[1:] {
		if !deletableDuplicateVolume(duplicate) {
			continue
		}
		if err := d.cloud.DeleteVolume(ctx, duplicate.ID); err != nil && !errors.Is(err, cloud.ErrNotFound) {
			if errors.Is(err, cloud.ErrConflict) {
				continue
			}
			return nil, err
		}
	}
	return &canonical, nil
}

func deletableDuplicateVolume(volume cloud.Volume) bool {
	return len(volume.Attachments) == 0 && volume.Status != cloud.VolumeStatusInUse
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return cloud.ErrUnavailable
	case <-timer.C:
		return nil
	}
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetNodeId() == "" {
		return d.unpublishAll(ctx, req.GetVolumeId())
	}
	zone := d.config.AvailabilityZone
	volume, err := d.cloud.GetVolumeByID(ctx, req.GetVolumeId())
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, statusError(err)
	}
	if volume.AvailabilityZone != "" {
		zone = volume.AvailabilityZone
	}
	if err := d.cloud.DetachVolume(ctx, req.GetVolumeId(), req.GetNodeId(), zone); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, statusError(err)
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) unpublishAll(ctx context.Context, volumeID string) (*csi.ControllerUnpublishVolumeResponse, error) {
	volume, err := d.cloud.GetVolumeByID(ctx, volumeID)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, statusError(err)
	}
	for _, attachment := range volume.Attachments {
		if err := d.cloud.DetachVolume(ctx, volumeID, attachment.InstanceID, volume.AvailabilityZone); err != nil {
			if errors.Is(err, cloud.ErrNotFound) {
				continue
			}
			return nil, statusError(err)
		}
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) ValidateVolumeCapabilities(_ context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if message := capabilityValidationMessage(req.GetVolumeCapabilities()); message != "" {
		return &csi.ValidateVolumeCapabilitiesResponse{
			Message: message,
		}, nil
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
			VolumeContext:      req.GetVolumeContext(),
		},
	}, nil
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	volumes, token, err := d.cloud.ListVolumes(ctx, cloud.Page{Token: req.GetStartingToken(), Size: int(req.GetMaxEntries())})
	if err != nil {
		return nil, statusError(err)
	}
	entries := make([]*csi.ListVolumesResponse_Entry, 0, len(volumes))
	for _, volume := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{Volume: csiVolume(volume)})
	}
	return &csi.ListVolumesResponse{Entries: entries, NextToken: token}, nil
}

func (d *Driver) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return &csi.GetCapacityResponse{AvailableCapacity: 0}, nil
}

func (d *Driver) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	types := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_GET_VOLUME,
	}
	capabilities := make([]*csi.ControllerServiceCapability, 0, len(types))
	for _, capability := range types {
		capabilities = append(capabilities, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{Type: capability},
			},
		})
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: capabilities}, nil
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	size, err := requestedBytes(req.GetCapacityRange(), d.config.MinimumVolumeBytes, d.config.VolumeGranularityBytes)
	if err != nil {
		return nil, err
	}
	volume, err := d.cloud.GetVolumeByID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, statusError(err)
	}
	allocated, err := d.cloud.ResizeVolume(ctx, req.GetVolumeId(), size, volume.AvailabilityZone)
	if err != nil {
		return nil, statusError(err)
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         allocated,
		NodeExpansionRequired: true,
	}, nil
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	volume, err := d.cloud.GetVolumeByID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, statusError(err)
	}
	return &csi.ControllerGetVolumeResponse{Volume: csiVolume(*volume)}, nil
}

func csiVolume(volume cloud.Volume) *csi.Volume {
	return &csi.Volume{
		VolumeId:      volume.ID,
		CapacityBytes: volume.SizeBytes,
		VolumeContext: volumeContext(volume),
		AccessibleTopology: []*csi.Topology{
			{Segments: map[string]string{
				TopologyZoneKey: volume.AvailabilityZone,
			}},
		},
	}
}
