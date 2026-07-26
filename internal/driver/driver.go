package driver

import (
	"log/slog"

	"github.com/brimble/nobus-csi/internal/cloud"
	"github.com/brimble/nobus-csi/internal/mount"
	"github.com/container-storage-interface/spec/lib/go/csi"
)

type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer

	config Config
	cloud  cloud.Cloud
	mount  mount.Interface
	locks  *VolumeLocks
	names  *VolumeLocks
	log    *slog.Logger
	ready  bool
}

func New(config Config, provider cloud.Cloud, mounter mount.Interface) *Driver {
	return &Driver{
		config: config,
		cloud:  provider,
		mount:  mounter,
		locks:  NewVolumeLocks(),
		names:  NewVolumeLocks(),
		log:    config.Logger,
		ready:  true,
	}
}
