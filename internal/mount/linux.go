//go:build linux

package mount

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	kmount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

const (
	deviceWaitInterval = 500 * time.Millisecond
	deviceWaitTimeout  = 2 * time.Minute
)

type Linux struct {
	mounter *kmount.SafeFormatAndMount
	exec    utilexec.Interface
}

func NewLinux() *Linux {
	exec := utilexec.New()
	return &Linux{
		mounter: &kmount.SafeFormatAndMount{
			Interface: kmount.New(""),
			Exec:      exec,
		},
		exec: exec,
	}
}

func (l *Linux) IsMounted(ctx context.Context, target string) (bool, error) {
	notMount, err := l.mounter.Interface.IsLikelyNotMountPoint(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect mount %s: %w", target, err)
	}
	return !notMount, nil
}

func (l *Linux) Stage(ctx context.Context, source string, target string, fsType string, flags []string) error {
	if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("create staging path %s: %w", target, err)
	}
	return l.withContext(ctx, func() error {
		if err := l.mounter.FormatAndMountSensitive(source, target, fsType, flags, nil); err != nil {
			return fmt.Errorf("stage device %s at %s: %w", source, target, err)
		}
		return nil
	})
}

func (l *Linux) Publish(ctx context.Context, source string, target string, readonly bool, block bool, flags []string) error {
	if block {
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create block target parent %s: %w", target, err)
		}
		file, err := os.OpenFile(target, os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("create block target %s: %w", target, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close block target %s: %w", target, err)
		}
	} else if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("create publish path %s: %w", target, err)
	}
	options := append([]string{"bind"}, flags...)
	if readonly {
		options = append(options, "ro")
	}
	return l.withContext(ctx, func() error {
		if err := l.mounter.Interface.MountSensitive(source, target, "", options, nil); err != nil {
			return fmt.Errorf("publish %s at %s: %w", source, target, err)
		}
		return nil
	})
}

func (l *Linux) Unmount(ctx context.Context, target string) error {
	mounted, err := l.IsMounted(ctx, target)
	if err != nil {
		return err
	}
	if !mounted {
		return nil
	}
	return l.withContext(ctx, func() error {
		if err := l.mounter.Interface.Unmount(target); err != nil {
			return fmt.Errorf("unmount %s: %w", target, err)
		}
		return nil
	})
}

func (l *Linux) Stats(_ context.Context, target string, block bool) (*Stats, error) {
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("volume path %s: %w", target, os.ErrNotExist)
		}
		return nil, fmt.Errorf("stat volume path %s: %w", target, err)
	}
	if block {
		return &Stats{TotalBytes: info.Size()}, nil
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(target, &stat); err != nil {
		return nil, fmt.Errorf("statfs %s: %w", target, err)
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	files := int64(stat.Files)
	freeFiles := int64(stat.Ffree)
	return &Stats{
		TotalBytes:     total,
		AvailableBytes: free,
		UsedBytes:      total - free,
		TotalInodes:    files,
		FreeInodes:     freeFiles,
		UsedInodes:     files - freeFiles,
	}, nil
}

func (l *Linux) Expand(ctx context.Context, target string, fsType string) error {
	switch fsType {
	case "ext4", "ext3", "ext2":
		return l.run(ctx, "resize2fs", target)
	case "xfs":
		return l.run(ctx, "xfs_growfs", target)
	default:
		return fmt.Errorf("unsupported filesystem %q", fsType)
	}
}

func (l *Linux) FindDevice(ctx context.Context, volumeID string, hintedPath string) (string, error) {
	candidates := []string{
		hintedPath,
		filepath.Join("/dev/disk/by-id", "virtio-"+volumeID),
		filepath.Join("/dev/disk/by-id", "scsi-0QEMU_QEMU_HARDDISK_"+volumeID),
	}
	deadline := time.NewTimer(deviceWaitTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(deviceWaitInterval)
	defer ticker.Stop()
	for {
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			resolved, err := filepath.EvalSymlinks(candidate)
			if err == nil {
				return resolved, nil
			}
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("resolve device %s: %w", candidate, err)
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for device %s: %w", volumeID, ctx.Err())
		case <-deadline.C:
			return "", fmt.Errorf("wait for device %s: %w", volumeID, os.ErrNotExist)
		case <-ticker.C:
		}
	}
}

func (l *Linux) withContext(ctx context.Context, action func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- action()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Linux) run(ctx context.Context, command string, args ...string) error {
	cmd := l.exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run %s: %w: %s", command, err, string(output))
	}
	return nil
}
