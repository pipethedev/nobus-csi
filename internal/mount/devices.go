package mount

import "path/filepath"

func deviceCandidates(volumeID string, hintedPath string) []string {
	return []string{
		hintedPath,
		filepath.Join("/dev/disk/by-id", "virtio-"+volumeID),
		filepath.Join("/dev/disk/by-id", "scsi-0QEMU_QEMU_HARDDISK_"+volumeID),
	}
}
