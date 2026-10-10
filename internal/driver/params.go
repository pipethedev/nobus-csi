package driver

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	paramAvailabilityZone = "availability_zone"
	paramProjectID        = "project_id"
	paramVolumeType       = "volume_type"
	paramEncrypted        = "encrypted"
	paramRegion           = "region"
	TopologyZoneKey       = "topology.csi.nobus.io/zone"
	TopologyRegionKey     = "topology.csi.nobus.io/region"
)

var supportedParameters = []string{
	paramAvailabilityZone,
	paramProjectID,
	paramRegion,
	paramVolumeType,
	paramEncrypted,
}

func volumeSpec(req *csi.CreateVolumeRequest, defaults Config) (cloud.VolumeSpec, error) {
	if req.GetName() == "" {
		return cloud.VolumeSpec{}, status.Error(codes.InvalidArgument, "volume name is required")
	}
	for key := range req.GetParameters() {
		if !slices.Contains(supportedParameters, key) {
			return cloud.VolumeSpec{}, status.Errorf(codes.InvalidArgument, "unknown parameter %q", key)
		}
	}
	size, err := requestedBytes(req.GetCapacityRange(), defaults.MinimumVolumeBytes, defaults.VolumeGranularityBytes)
	if err != nil {
		return cloud.VolumeSpec{}, err
	}
	params := req.GetParameters()
	zone := selectedZone(req, cmp.Or(params[paramAvailabilityZone], defaults.AvailabilityZone))
	project := cmp.Or(params[paramProjectID], defaults.ProjectID)
	if zone == "" {
		return cloud.VolumeSpec{}, status.Error(codes.InvalidArgument, "availability_zone is required")
	}
	if project == "" {
		return cloud.VolumeSpec{}, status.Error(codes.InvalidArgument, "project_id is required")
	}
	encrypted := false
	if raw, ok := params[paramEncrypted]; ok {
		encrypted, err = strconv.ParseBool(raw)
		if err != nil {
			return cloud.VolumeSpec{}, status.Error(codes.InvalidArgument, "encrypted must be true or false")
		}
	}
	return cloud.VolumeSpec{
		Name:             req.GetName(),
		SizeBytes:        size,
		AvailabilityZone: zone,
		ProjectID:        project,
		Type:             cmp.Or(params[paramVolumeType], defaults.VolumeType),
		SnapshotID:       snapshotSource(req, defaults.AvailabilityZone),
		Encrypted:        encrypted,
		Metadata:         map[string]string{"csi.driver": defaults.DriverName},
	}, nil
}

func selectedZone(req *csi.CreateVolumeRequest, fallback string) string {
	requirements := req.GetAccessibilityRequirements()
	requisite := topologyZones(requirements.GetRequisite())
	for _, topology := range requirements.GetPreferred() {
		if zone := topology.GetSegments()[TopologyZoneKey]; zone != "" {
			if len(requisite) > 0 && !slices.Contains(requisite, zone) {
				continue
			}
			return zone
		}
	}
	if len(requisite) > 0 {
		if fallback != "" && slices.Contains(requisite, fallback) {
			return fallback
		}
		return requisite[0]
	}
	return fallback
}

func topologyZones(topologies []*csi.Topology) []string {
	zones := make([]string, 0, len(topologies))
	for _, topology := range topologies {
		zone := topology.GetSegments()[TopologyZoneKey]
		if zone != "" && !slices.Contains(zones, zone) {
			zones = append(zones, zone)
		}
	}
	return zones
}

func requestedBytes(capacity *csi.CapacityRange, minimumBytes int64, granularityBytes int64) (int64, error) {
	required := capacity.GetRequiredBytes()
	if required == 0 {
		required = minimumBytes
	}
	rounded := roundUp(required, granularityBytes)
	limit := capacity.GetLimitBytes()
	if limit > 0 && rounded > limit {
		return 0, status.Errorf(codes.OutOfRange, "requested capacity %d exceeds limit %d after rounding", rounded, limit)
	}
	return rounded, nil
}

func roundUp(value int64, granularity int64) int64 {
	if granularity <= 0 || value%granularity == 0 {
		return value
	}
	return ((value / granularity) + 1) * granularity
}

func snapshotSource(req *csi.CreateVolumeRequest, fallbackZone string) string {
	source := req.GetVolumeContentSource()
	if source == nil {
		return ""
	}
	snapshot := source.GetSnapshot()
	if snapshot == nil {
		return ""
	}
	id, _ := decodeSnapshotID(snapshot.GetSnapshotId(), fallbackZone)
	return id
}

func capabilityMessage(capabilities []*csi.VolumeCapability) string {
	if err := validateCapabilities(capabilities); err != nil {
		return err.Error()
	}
	return ""
}

func validateCapabilities(capabilities []*csi.VolumeCapability) error {
	if len(capabilities) == 0 {
		return status.Error(codes.InvalidArgument, "volume capabilities are required")
	}
	for index, capability := range capabilities {
		if capability == nil {
			return status.Errorf(codes.InvalidArgument, "volume capability %d is empty", index)
		}
		mode := capability.GetAccessMode().GetMode()
		switch mode {
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		default:
			return status.Errorf(codes.InvalidArgument, "unsupported access mode %s", mode.String())
		}
		if capability.GetMount() == nil && capability.GetBlock() == nil {
			return status.Errorf(codes.InvalidArgument, "volume capability %d requires mount or block access type", index)
		}
	}
	return nil
}

func volumeContext(volume cloud.Volume) map[string]string {
	context := map[string]string{
		paramAvailabilityZone: volume.AvailabilityZone,
	}
	if volume.Type != "" {
		context[paramVolumeType] = volume.Type
	}
	context[paramEncrypted] = strconv.FormatBool(volume.Encrypted)
	return context
}
