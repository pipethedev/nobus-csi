package mount

import (
	"context"
	"sync"
)

type Fake struct {
	mu      sync.Mutex
	mounted map[string]string
}

func NewFake() *Fake {
	return &Fake{mounted: make(map[string]string)}
}

func (f *Fake) IsMounted(_ context.Context, target string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.mounted[target]
	return ok, nil
}

func (f *Fake) Stage(_ context.Context, source string, target string, _ string, _ []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mounted[target] = source
	return nil
}

func (f *Fake) Publish(_ context.Context, source string, target string, _ bool, _ bool, _ []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mounted[target] = source
	return nil
}

func (f *Fake) Unmount(_ context.Context, target string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mounted, target)
	return nil
}

func (f *Fake) Stats(context.Context, string, bool) (*Stats, error) {
	return &Stats{TotalBytes: 1 << 30, AvailableBytes: 1 << 29, UsedBytes: 1 << 29}, nil
}

func (f *Fake) Expand(context.Context, string, string) error {
	return nil
}

func (f *Fake) FindDevice(_ context.Context, volumeID string, hintedPath string) (string, error) {
	if hintedPath != "" {
		return hintedPath, nil
	}
	return "/dev/disk/by-id/virtio-" + volumeID, nil
}
