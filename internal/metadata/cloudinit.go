package metadata

import (
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

func ReadCloudInit(ctx context.Context) (*Instance, error) {
	for _, path := range cloudInitPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		instance, err := readPath(path)
		if err == nil {
			return instance, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("cloud-init instance metadata: %w", os.ErrNotExist)
}

func readPath(path string) (*Instance, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer closeFile(file)
	var data cloudInitData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	instance := Instance{
		ID:               firstValue(data.V1.InstanceID, data.V1.InstanceIDHyphen, data.DS.MetaData.InstanceID, data.DS.MetaData.InstanceIDHyphen),
		AvailabilityZone: firstValue(data.V1.AvailabilityZone, data.V1.AvailabilityZoneHyphen, data.DS.MetaData.AvailabilityZone),
		Region:           firstValue(data.V1.Region, data.DS.MetaData.Region),
	}
	if instance.ID == "" {
		return nil, fmt.Errorf("cloud-init instance id missing: %w", os.ErrInvalid)
	}
	return &instance, nil
}

func closeFile(file *os.File) {
	_ = file.Close()
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
