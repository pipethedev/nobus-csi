//go:build linux

package mount

func New() Mounter {
	return newLinux()
}
