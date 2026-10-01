package nobusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Snapshot struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	VolumeID    string            `json:"volume_id"`
	Status      string            `json:"status"`
	Size        int               `json:"size"`
	CreatedAt   string            `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
}

type CreateSnapshotRequest struct {
	VolumeID         string            `json:"volume_id"`
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	Force            bool              `json:"force,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
}

func (c *Client) CreateSnapshot(ctx context.Context, req CreateSnapshotRequest) (*Snapshot, error) {
	var resp struct {
		Data Snapshot `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/volume/snapshot", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListSnapshots(ctx context.Context, projectID, availabilityZone string) ([]Snapshot, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/volume/snapshot/list", projectQuery(projectID, availabilityZone), nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Snapshot](resp.Data)
}

func (c *Client) GetSnapshot(ctx context.Context, projectID, availabilityZone, snapshotID string) (*Snapshot, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("snapshot_id", snapshotID)
	var resp struct {
		Data Snapshot `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/volume/snapshot", q, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data.ID == "" {
		return nil, fmt.Errorf("snapshot %s: %w", snapshotID, ErrNotFound)
	}
	return &resp.Data, nil
}

func (c *Client) DeleteSnapshot(ctx context.Context, projectID, availabilityZone, snapshotID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("snapshot_id", snapshotID)
	return c.do(ctx, http.MethodDelete, "/api/v3/volume/snapshot", q, nil, nil)
}

func (c *Client) WaitSnapshotStatus(ctx context.Context, projectID, availabilityZone, snapshotID string, want ...string) (*Snapshot, error) {
	wanted := map[string]struct{}{}
	for _, status := range want {
		wanted[strings.ToLower(status)] = struct{}{}
	}
	var current *Snapshot
	err := c.waitUntil(ctx, 5*time.Minute, func(ctx context.Context) (bool, error) {
		snapshot, err := c.GetSnapshot(ctx, projectID, availabilityZone, snapshotID)
		if err != nil {
			return false, err
		}
		current = snapshot
		_, ok := wanted[strings.ToLower(snapshot.Status)]
		return ok, nil
	})
	return current, err
}
