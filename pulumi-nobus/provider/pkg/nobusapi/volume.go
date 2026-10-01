package nobusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type VolumeAttachment struct {
	ServerID string `json:"server_id"`
	Device   string `json:"device"`
}

type Volume struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Size             int                `json:"size"`
	Status           string             `json:"status"`
	VolumeType       string             `json:"volume_type"`
	AvailabilityZone string             `json:"availability_zone"`
	Multiattach      bool               `json:"multiattach"`
	Metadata         map[string]string  `json:"metadata"`
	Attachments      []VolumeAttachment `json:"attachments"`
}

type CreateVolumeRequest struct {
	Size             int               `json:"size"`
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	VolumeType       string            `json:"volume_type,omitempty"`
	AvailabilityZone string            `json:"availability_zone"`
	SnapshotID       string            `json:"snapshot_id,omitempty"`
	SourceVolID      string            `json:"source_volid,omitempty"`
	ImageRef         string            `json:"image_ref,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Multiattach      *bool             `json:"multiattach,omitempty"`
	ProjectID        string            `json:"project_id"`
}

type UpdateVolumeRequest struct {
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
}

type AttachVolumeRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
	VolumeID         string `json:"volume_id"`
}

func (c *Client) CreateVolume(ctx context.Context, req CreateVolumeRequest) (*Volume, error) {
	var resp struct {
		Status bool   `json:"status"`
		Data   Volume `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/volume/", nil, req, &resp); err != nil {
		return nil, err
	}
	if resp.Data.AvailabilityZone == "" {
		resp.Data.AvailabilityZone = req.AvailabilityZone
	}
	return &resp.Data, nil
}

func (c *Client) ListVolumes(ctx context.Context, projectID, availabilityZone string) ([]Volume, error) {
	q := projectQuery(projectID, availabilityZone)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/volume/list", q, nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Volume](resp.Data)
}

func (c *Client) GetVolume(ctx context.Context, projectID, availabilityZone, volumeID string) (*Volume, error) {
	// Nobus GET-by-id is unreliable; list and filter like the CSI driver.
	volumes, err := c.ListVolumes(ctx, projectID, availabilityZone)
	if err != nil {
		return nil, err
	}
	for _, volume := range volumes {
		if volume.ID == volumeID {
			if volume.AvailabilityZone == "" {
				volume.AvailabilityZone = availabilityZone
			}
			return &volume, nil
		}
	}
	return nil, fmt.Errorf("volume %s: %w", volumeID, ErrNotFound)
}

func (c *Client) UpdateVolume(ctx context.Context, volumeID string, req UpdateVolumeRequest) (*Volume, error) {
	q := projectQuery(req.ProjectID, req.AvailabilityZone)
	q.Set("volume_id", volumeID)
	var resp struct {
		Status bool   `json:"status"`
		Data   Volume `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/volume/", q, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteVolume(ctx context.Context, projectID, availabilityZone, volumeID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("volume_id", volumeID)
	return c.do(ctx, http.MethodDelete, "/api/v3/volume/", q, nil, nil)
}

func (c *Client) AttachVolume(ctx context.Context, req AttachVolumeRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/volume/attach", nil, req, nil)
}

func (c *Client) DetachVolume(ctx context.Context, req AttachVolumeRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/volume/detach", nil, req, nil)
}

func (c *Client) WaitVolumeStatus(ctx context.Context, projectID, availabilityZone, volumeID string, want ...string) (*Volume, error) {
	wanted := map[string]struct{}{}
	for _, status := range want {
		wanted[strings.ToLower(status)] = struct{}{}
	}
	var current *Volume
	err := c.waitUntil(ctx, 5*time.Minute, func(ctx context.Context) (bool, error) {
		volume, err := c.GetVolume(ctx, projectID, availabilityZone, volumeID)
		if err != nil {
			return false, err
		}
		current = volume
		_, ok := wanted[strings.ToLower(volume.Status)]
		return ok, nil
	})
	return current, err
}

func (c *Client) WaitVolumeAttached(ctx context.Context, projectID, availabilityZone, volumeID, serverID string) (string, error) {
	var device string
	err := c.waitUntil(ctx, 3*time.Minute, func(ctx context.Context) (bool, error) {
		volume, err := c.GetVolume(ctx, projectID, availabilityZone, volumeID)
		if err != nil {
			return false, err
		}
		for _, attachment := range volume.Attachments {
			if attachment.ServerID == serverID && attachment.Device != "" {
				device = attachment.Device
				return true, nil
			}
		}
		return false, nil
	})
	return device, err
}
