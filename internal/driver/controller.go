package driver

import (
	"context"
	"errors"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	existing, err := d.cloud.GetVolumeByName(ctx, req.GetName())
	if err == nil {
		if existing.SizeBytes != spec.SizeBytes || existing.AvailabilityZone != spec.AvailabilityZone || existing.Type != spec.Type {
			return nil, status.Error(codes.AlreadyExists, "volume name exists with different parameters")
		}
		return &csi.CreateVolumeResponse{Volume: csiVolume(*existing)}, nil
	}
	if !errors.Is(err, cloud.ErrNotFound) {
		return nil, statusError(err)
	}
	volume, err := d.cloud.CreateVolume(ctx, spec)
	if err != nil {
		if errors.Is(err, cloud.ErrAlreadyExists) {
			reconciled, getErr := d.cloud.GetVolumeByName(ctx, req.GetName())
			if getErr == nil {
				return &csi.CreateVolumeResponse{Volume: csiVolume(*reconciled)}, nil
			}
		}
		return nil, statusError(err)
	}
	return &csi.CreateVolumeResponse{Volume: csiVolume(*volume)}, nil
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
	if err == nil {
		device, attachedElsewhere := attachedDevice(*volume, req.GetNodeId())
		if device != "" {
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{"device_path": device},
			}, nil
		}
		if attachedElsewhere {
			return nil, statusError(cloud.ErrConflict)
		}
	} else if !errors.Is(err, cloud.ErrNotFound) {
		return nil, statusError(err)
	}
	device, err := d.cloud.AttachVolume(ctx, req.GetVolumeId(), req.GetNodeId())
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

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetNodeId() == "" {
		return d.unpublishAll(ctx, req.GetVolumeId())
	}
	if err := d.cloud.DetachVolume(ctx, req.GetVolumeId(), req.GetNodeId()); err != nil {
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
		if err := d.cloud.DetachVolume(ctx, volumeID, attachment.InstanceID); err != nil {
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
	allocated, err := d.cloud.ResizeVolume(ctx, req.GetVolumeId(), size)
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
