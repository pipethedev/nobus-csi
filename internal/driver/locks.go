package driver

import "sync"

type VolumeLocks struct {
	mu    sync.Mutex
	locks map[string]struct{}
}

func NewVolumeLocks() *VolumeLocks {
	return &VolumeLocks{locks: make(map[string]struct{})}
}

func (l *VolumeLocks) TryLock(id string) func() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.locks[id]; ok {
		return nil
	}
	l.locks[id] = struct{}{}
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.locks, id)
	}
}
