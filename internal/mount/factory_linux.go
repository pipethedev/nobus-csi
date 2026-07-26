//go:build linux

package mount

func New() Interface {
	return NewLinux()
}
