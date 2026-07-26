package mount

import "context"

type Interface interface {
	IsMounted(ctx context.Context, target string) (bool, error)
	Stage(ctx context.Context, source string, target string, fsType string, flags []string) error
	Publish(ctx context.Context, source string, target string, readonly bool, block bool, flags []string) error
	Unmount(ctx context.Context, target string) error
	Stats(ctx context.Context, target string, block bool) (*Stats, error)
	Expand(ctx context.Context, target string, fsType string) error
	FindDevice(ctx context.Context, volumeID string, hintedPath string) (string, error)
}

type Stats struct {
	TotalBytes     int64
	AvailableBytes int64
	UsedBytes      int64
	TotalInodes    int64
	FreeInodes     int64
	UsedInodes     int64
}
