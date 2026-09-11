package cloud

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
)

type Fake struct {
	mu           sync.Mutex
	volumes      map[string]Volume
	snapshots    map[string]Snapshot
	metadata     InstanceMetadata
	nextVolume   int
	nextSnapshot int
}

func NewFake(metadata InstanceMetadata) *Fake {
	return &Fake{
		volumes:      make(map[string]Volume),
		snapshots:    make(map[string]Snapshot),
		metadata:     metadata,
		nextVolume:   1,
		nextSnapshot: 1,
	}
}

func (f *Fake) CreateVolume(_ context.Context, spec VolumeSpec) (*Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, volume := range f.volumes {
		if volume.Name == spec.Name {
			if volume.SizeBytes != spec.SizeBytes || volume.AvailabilityZone != spec.AvailabilityZone || volume.Type != spec.Type {
				return nil, ErrAlreadyExists
			}
			clone := volume
			return &clone, nil
		}
	}
	id := fmt.Sprintf("vol-%d", f.nextVolume)
	f.nextVolume++
	volume := Volume{
		ID:               id,
		Name:             spec.Name,
		SizeBytes:        spec.SizeBytes,
		Status:           VolumeStatusAvailable,
		AvailabilityZone: spec.AvailabilityZone,
		Type:             spec.Type,
		Metadata:         maps.Clone(spec.Metadata),
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

func (f *Fake) ListVolumes(_ context.Context, page Page) ([]Volume, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	volumes := make([]Volume, 0, len(f.volumes))
	for _, volume := range f.volumes {
		if page.AvailabilityZone != "" && volume.AvailabilityZone != page.AvailabilityZone {
			continue
		}
		if page.Name != "" && volume.Name != page.Name {
			continue
		}
		volumes = append(volumes, volume)
	}
	slices.SortFunc(volumes, func(left Volume, right Volume) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return paginate(volumes, page)
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
			if snapshot.VolumeID != spec.VolumeID {
				return nil, ErrAlreadyExists
			}
			clone := snapshot
			return &clone, nil
		}
	}
	id := fmt.Sprintf("snap-%d", f.nextSnapshot)
	f.nextSnapshot++
	snapshot := Snapshot{
		ID:               id,
		Name:             spec.Name,
		VolumeID:         spec.VolumeID,
		SizeBytes:        volume.SizeBytes,
		Status:           SnapshotStatusAvailable,
		CreatedAt:        time.Now().UTC(),
		AvailabilityZone: spec.AvailabilityZone,
		Metadata:         maps.Clone(spec.Metadata),
	}
	f.snapshots[id] = snapshot
	return &snapshot, nil
}

func (f *Fake) DeleteSnapshot(_ context.Context, id string, availabilityZone string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot, ok := f.snapshots[id]
	if !ok {
		return nil
	}
	if availabilityZone != "" && snapshot.AvailabilityZone != availabilityZone {
		return ErrNotFound
	}
	delete(f.snapshots, id)
	return nil
}

func (f *Fake) ListSnapshots(_ context.Context, page Page) ([]Snapshot, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshots := make([]Snapshot, 0, len(f.snapshots))
	for _, snapshot := range f.snapshots {
		if page.AvailabilityZone != "" && snapshot.AvailabilityZone != page.AvailabilityZone {
			continue
		}
		if page.Name != "" && snapshot.Name != page.Name {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	slices.SortFunc(snapshots, func(left Snapshot, right Snapshot) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return paginate(snapshots, page)
}

func (f *Fake) GetInstanceMetadata(_ context.Context) (*InstanceMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	metadata := f.metadata
	return &metadata, nil
}
