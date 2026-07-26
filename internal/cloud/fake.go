package cloud

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

type Fake struct {
	mu        sync.Mutex
	volumes   map[string]Volume
	snapshots map[string]Snapshot
	metadata  InstanceMetadata
	failures  map[string]error
}

func NewFake(metadata InstanceMetadata) *Fake {
	return &Fake{
		volumes:   make(map[string]Volume),
		snapshots: make(map[string]Snapshot),
		metadata:  metadata,
		failures:  make(map[string]error),
	}
}

func (f *Fake) SetFailure(operation string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[operation] = err
}

func (f *Fake) CreateVolume(_ context.Context, spec VolumeSpec) (*Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failure("CreateVolume"); err != nil {
		return nil, err
	}
	for _, volume := range f.volumes {
		if volume.Name == spec.Name {
			if volume.SizeBytes != spec.SizeBytes || volume.AvailabilityZone != spec.AvailabilityZone || volume.Type != spec.Type {
				return nil, ErrAlreadyExists
			}
			clone := volume
			return &clone, nil
		}
	}
	id := fmt.Sprintf("vol-%d", len(f.volumes)+1)
	volume := Volume{
		ID:               id,
		Name:             spec.Name,
		SizeBytes:        spec.SizeBytes,
		Status:           VolumeStatusAvailable,
		AvailabilityZone: spec.AvailabilityZone,
		Type:             spec.Type,
		Multiattach:      false,
		Metadata:         copyMap(spec.Metadata),
	}
	f.volumes[id] = volume
	return &volume, nil
}

func (f *Fake) GetVolumeByID(_ context.Context, id string) (*Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volume, ok := f.volumes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &volume, nil
}

func (f *Fake) GetVolumeByName(_ context.Context, name string, availabilityZone string) (*Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, volume := range f.volumes {
		if volume.Name == name && volume.AvailabilityZone == availabilityZone {
			clone := volume
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (f *Fake) ListVolumes(_ context.Context, _ Page) ([]Volume, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volumes := make([]Volume, 0, len(f.volumes))
	for _, volume := range f.volumes {
		volumes = append(volumes, volume)
	}
	return volumes, "", nil
}

func (f *Fake) DeleteVolume(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.volumes, id)
	return nil
}

func (f *Fake) ResizeVolume(_ context.Context, id string, sizeBytes int64, availabilityZone string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volume, ok := f.volumes[id]
	if !ok {
		return 0, ErrNotFound
	}
	if availabilityZone != "" && volume.AvailabilityZone != availabilityZone {
		return 0, ErrNotFound
	}
	if sizeBytes < volume.SizeBytes {
		return 0, ErrOutOfRange
	}
	volume.SizeBytes = sizeBytes
	f.volumes[id] = volume
	return sizeBytes, nil
}

func (f *Fake) AttachVolume(_ context.Context, volumeID, instanceID string, availabilityZone string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volume, ok := f.volumes[volumeID]
	if !ok {
		return "", ErrNotFound
	}
	if availabilityZone != "" && volume.AvailabilityZone != availabilityZone {
		return "", ErrNotFound
	}
	if len(volume.Attachments) > 0 {
		attachment := volume.Attachments[0]
		if attachment.InstanceID != instanceID {
			return "", ErrConflict
		}
		return attachment.DevicePath, nil
	}
	device := fmt.Sprintf("/dev/disk/by-id/virtio-%s", volumeID)
	volume.Attachments = append(volume.Attachments, Attachment{InstanceID: instanceID, DevicePath: device})
	volume.Status = VolumeStatusInUse
	f.volumes[volumeID] = volume
	return device, nil
}

func (f *Fake) DetachVolume(_ context.Context, volumeID, instanceID string, availabilityZone string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	volume, ok := f.volumes[volumeID]
	if !ok {
		return nil
	}
	if availabilityZone != "" && volume.AvailabilityZone != availabilityZone {
		return nil
	}
	volume.Attachments = slices.DeleteFunc(volume.Attachments, func(attachment Attachment) bool {
		return attachment.InstanceID == instanceID
	})
	if len(volume.Attachments) == 0 {
		volume.Status = VolumeStatusAvailable
	}
	f.volumes[volumeID] = volume
	return nil
}

func (f *Fake) CreateSnapshot(_ context.Context, spec SnapshotSpec) (*Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volume, ok := f.volumes[spec.VolumeID]
	if !ok {
		return nil, ErrNotFound
	}
	for _, snapshot := range f.snapshots {
		if snapshot.Name == spec.Name {
			clone := snapshot
			return &clone, nil
		}
	}
	id := fmt.Sprintf("snap-%d", len(f.snapshots)+1)
	snapshot := Snapshot{
		ID:        id,
		Name:      spec.Name,
		VolumeID:  spec.VolumeID,
		SizeBytes: volume.SizeBytes,
		Status:    SnapshotStatusAvailable,
		Metadata:  copyMap(spec.Metadata),
	}
	f.snapshots[id] = snapshot
	return &snapshot, nil
}

func (f *Fake) DeleteSnapshot(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.snapshots, id)
	return nil
}

func (f *Fake) ListSnapshots(_ context.Context, _ Page) ([]Snapshot, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshots := make([]Snapshot, 0, len(f.snapshots))
	for _, snapshot := range f.snapshots {
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, "", nil
}

func (f *Fake) GetInstanceMetadata(_ context.Context) (*InstanceMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	metadata := f.metadata
	return &metadata, nil
}

func (f *Fake) failure(operation string) error {
	err, ok := f.failures[operation]
	if !ok {
		return nil
	}
	delete(f.failures, operation)
	return err
}

func copyMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
