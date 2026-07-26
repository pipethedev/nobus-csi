package cloud

import "context"

type Cloud interface {
	CreateVolume(ctx context.Context, spec VolumeSpec) (*Volume, error)
	GetVolumeByID(ctx context.Context, id string) (*Volume, error)
	GetVolumeByName(ctx context.Context, name string, availabilityZone string) (*Volume, error)
	ListVolumes(ctx context.Context, page Page) ([]Volume, string, error)
	DeleteVolume(ctx context.Context, id string) error
	ResizeVolume(ctx context.Context, id string, sizeBytes int64, availabilityZone string) (int64, error)
	AttachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) (string, error)
	DetachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) error
	CreateSnapshot(ctx context.Context, spec SnapshotSpec) (*Snapshot, error)
	DeleteSnapshot(ctx context.Context, id string) error
	ListSnapshots(ctx context.Context, page Page) ([]Snapshot, string, error)
	GetInstanceMetadata(ctx context.Context) (*InstanceMetadata, error)
}

type Page struct {
	Token string
	Size  int
}

type VolumeSpec struct {
	Name             string
	SizeBytes        int64
	AvailabilityZone string
	ProjectID        string
	Type             string
	SnapshotID       string
	Metadata         map[string]string
}

type Volume struct {
	ID               string
	Name             string
	SizeBytes        int64
	Status           VolumeStatus
	AvailabilityZone string
	Region           string
	Type             string
	Multiattach      bool
	Metadata         map[string]string
	Attachments      []Attachment
}

type Attachment struct {
	InstanceID string
	DevicePath string
}

type VolumeStatus string

const (
	VolumeStatusAvailable VolumeStatus = "available"
	VolumeStatusInUse     VolumeStatus = "in-use"
	VolumeStatusCreating  VolumeStatus = "creating"
	VolumeStatusDeleting  VolumeStatus = "deleting"
	VolumeStatusExtending VolumeStatus = "extending"
	VolumeStatusError     VolumeStatus = "error"
)

type SnapshotSpec struct {
	Name             string
	VolumeID         string
	ProjectID        string
	AvailabilityZone string
	Force            bool
	Metadata         map[string]string
}

type Snapshot struct {
	ID        string
	Name      string
	VolumeID  string
	SizeBytes int64
	Status    SnapshotStatus
	Metadata  map[string]string
}

type SnapshotStatus string

const (
	SnapshotStatusAvailable SnapshotStatus = "available"
	SnapshotStatusCreating  SnapshotStatus = "creating"
	SnapshotStatusDeleting  SnapshotStatus = "deleting"
	SnapshotStatusError     SnapshotStatus = "error"
)

type InstanceMetadata struct {
	InstanceID        string
	AvailabilityZone  string
	Region            string
	MaxVolumesPerNode int64
}
