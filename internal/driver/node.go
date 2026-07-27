package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultFSType = "ext4"

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}
	capability := req.GetVolumeCapability()
	if capability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}
	mounted, err := d.mount.IsMounted(ctx, req.GetStagingTargetPath())
	if err != nil {
		return nil, statusError(err)
	}
	if mounted {
		return &csi.NodeStageVolumeResponse{}, nil
	}
	device, err := d.mount.FindDevice(ctx, req.GetVolumeId(), req.GetPublishContext()["device_path"])
	if err != nil {
		return nil, statusError(err)
	}
	fsType := defaultFSType
	flags := []string(nil)
	if mount := capability.GetMount(); mount != nil {
		fsType = firstValue(mount.GetFsType(), defaultFSType)
		flags = mount.GetMountFlags()
	}
	if err := d.mount.Stage(ctx, device, req.GetStagingTargetPath(), fsType, flags); err != nil {
		return nil, statusError(err)
	}
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}
	if err := d.mount.Unmount(ctx, req.GetStagingTargetPath()); err != nil {
		return nil, statusError(err)
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}
	capability := req.GetVolumeCapability()
	if capability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}
	mounted, err := d.mount.IsMounted(ctx, req.GetTargetPath())
	if err != nil {
		return nil, statusError(err)
	}
	if mounted {
		return &csi.NodePublishVolumeResponse{}, nil
	}
	block := capability.GetBlock() != nil
	flags := []string(nil)
	if mount := capability.GetMount(); mount != nil {
		flags = mount.GetMountFlags()
	}
	source := req.GetStagingTargetPath()
	if block {
		device, err := d.mount.FindDevice(ctx, req.GetVolumeId(), req.GetPublishContext()["device_path"])
		if err != nil {
			return nil, statusError(err)
		}
		if device == "" {
			return nil, status.Error(codes.Unavailable, "block device is unavailable")
		}
		source = device
	}
	if err := d.mount.Publish(ctx, source, req.GetTargetPath(), req.GetReadonly(), block, flags); err != nil {
		return nil, statusError(err)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is required")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}
	if err := d.mount.Unmount(ctx, req.GetTargetPath()); err != nil {
		return nil, statusError(err)
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if req.GetVolumePath() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}
	block, err := d.mount.IsBlock(ctx, req.GetVolumePath())
	if err != nil {
		return nil, statusError(err)
	}
	stats, err := d.mount.Stats(ctx, req.GetVolumePath(), block)
	if err != nil {
		return nil, statusError(err)
	}
	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{Unit: csi.VolumeUsage_BYTES, Total: stats.TotalBytes, Available: stats.AvailableBytes, Used: stats.UsedBytes},
			{Unit: csi.VolumeUsage_INODES, Total: stats.TotalInodes, Available: stats.FreeInodes, Used: stats.UsedInodes},
		},
	}, nil
}

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	if req.GetVolumePath() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}
	fsType := defaultFSType
	if capability := req.GetVolumeCapability(); capability != nil {
		if mount := capability.GetMount(); mount != nil {
			fsType = firstValue(mount.GetFsType(), defaultFSType)
		}
	}
	if err := d.mount.Expand(ctx, req.GetVolumePath(), fsType); err != nil {
		return nil, statusError(err)
	}
	return &csi.NodeExpandVolumeResponse{CapacityBytes: req.GetCapacityRange().GetRequiredBytes()}, nil
}

func (d *Driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	types := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	}
	capabilities := make([]*csi.NodeServiceCapability, 0, len(types))
	for _, capability := range types {
		capabilities = append(capabilities, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{Type: capability},
			},
		})
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: capabilities}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	metadata, err := d.cloud.GetInstanceMetadata(ctx)
	if err != nil {
		return nil, statusError(err)
	}
	segments := map[string]string{}
	zone := nodeAvailabilityZone(metadata.AvailabilityZone, d.config.AvailabilityZone)
	if zone != "" {
		segments[TopologyZoneKey] = zone
	}
	if metadata.Region != "" {
		segments[TopologyRegionKey] = metadata.Region
	}
	resp := &csi.NodeGetInfoResponse{NodeId: metadata.InstanceID}
	if len(segments) > 0 {
		resp.AccessibleTopology = &csi.Topology{Segments: segments}
	}
	if metadata.MaxVolumesPerNode > 0 {
		resp.MaxVolumesPerNode = metadata.MaxVolumesPerNode
	}
	return resp, nil
}

func nodeAvailabilityZone(metadataZone string, configuredZone string) string {
	if configuredZone != "" && (metadataZone == "" || metadataZone == "nova") {
		return configuredZone
	}
	return metadataZone
}
