package metadata

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

var cloudInitPaths = []string{
	"/run/cloud-init/instance-data.json",
	"/var/lib/cloud/instance/instance-data.json",
}

type Instance struct {
	ID               string
	AvailabilityZone string
	Region           string
}

func ReadCloudInit(ctx context.Context) (Instance, error) {
	for _, path := range cloudInitPaths {
		if err := ctx.Err(); err != nil {
			return Instance{}, err
		}
		instance, err := readPath(path)
		if err == nil {
			return instance, nil
		}
		if !os.IsNotExist(err) {
			return Instance{}, err
		}
	}
	return Instance{}, fmt.Errorf("cloud-init instance metadata: %w", os.ErrNotExist)
}

func readPath(path string) (Instance, error) {
	file, err := os.Open(path)
	if err != nil {
		return Instance{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	var data cloudInitData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return Instance{}, fmt.Errorf("decode %s: %w", path, err)
	}
	instance := Instance{
		ID:               cmp.Or(data.V1.InstanceID, data.V1.InstanceIDHyphen, data.DS.MetaData.InstanceID, data.DS.MetaData.InstanceIDHyphen),
		AvailabilityZone: cmp.Or(data.V1.AvailabilityZone, data.V1.AvailabilityZoneHyphen, data.DS.MetaData.AvailabilityZone),
		Region:           cmp.Or(data.V1.Region, data.DS.MetaData.Region),
	}
	if instance.ID == "" {
		return Instance{}, fmt.Errorf("cloud-init instance id missing: %w", os.ErrInvalid)
	}
	return instance, nil
}

type cloudInitData struct {
	V1 cloudInitV1 `json:"v1"`
	DS cloudInitDS `json:"ds"`
}

type cloudInitV1 struct {
	InstanceID             string `json:"instance_id"`
	InstanceIDHyphen       string `json:"instance-id"`
	AvailabilityZone       string `json:"availability_zone"`
	AvailabilityZoneHyphen string `json:"availability-zone"`
	Region                 string `json:"region"`
}

type cloudInitDS struct {
	MetaData cloudInitMetaData `json:"meta_data"`
}

type cloudInitMetaData struct {
	InstanceID       string `json:"instance_id"`
	InstanceIDHyphen string `json:"instance-id"`
	AvailabilityZone string `json:"availability_zone"`
	Region           string `json:"region"`
}
