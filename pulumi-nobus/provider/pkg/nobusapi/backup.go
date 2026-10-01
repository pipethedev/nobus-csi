package nobusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Backup struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	VolumeID         string            `json:"volume_id"`
	Size             int               `json:"size"`
	Container        string            `json:"container"`
	Status           string            `json:"status"`
	CreatedAt        string            `json:"created_at"`
	AvailabilityZone string            `json:"availability_zone"`
	Metadata         map[string]string `json:"metadata"`
}

type CreateBackupRequest struct {
	VolumeID         string            `json:"volume_id"`
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	Container        string            `json:"container,omitempty"`
	Incremental      bool              `json:"incremental,omitempty"`
	Force            bool              `json:"force,omitempty"`
	SnapshotID       string            `json:"snapshot_id,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
}

func (c *Client) CreateBackup(ctx context.Context, req CreateBackupRequest) (*Backup, error) {
	var resp struct {
		Data Backup `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/volume/backup", nil, req, &resp); err != nil {
		return nil, err
	}
	if resp.Data.AvailabilityZone == "" {
		resp.Data.AvailabilityZone = req.AvailabilityZone
	}
	return &resp.Data, nil
}

func (c *Client) ListBackups(ctx context.Context, projectID, availabilityZone string) ([]Backup, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/volume/backup/list", projectQuery(projectID, availabilityZone), nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Backup](resp.Data)
}

func (c *Client) GetBackup(ctx context.Context, projectID, availabilityZone, backupID string) (*Backup, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("backup_id", backupID)
	var resp struct {
		Data Backup `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/volume/backup", q, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data.ID == "" {
		return nil, fmt.Errorf("backup %s: %w", backupID, ErrNotFound)
	}
	return &resp.Data, nil
}

func (c *Client) DeleteBackup(ctx context.Context, projectID, availabilityZone, backupID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("backup_id", backupID)
	return c.do(ctx, http.MethodDelete, "/api/v3/volume/backup", q, nil, nil)
}

func (c *Client) WaitBackupStatus(ctx context.Context, projectID, availabilityZone, backupID string, want ...string) (*Backup, error) {
	wanted := map[string]struct{}{}
	for _, status := range want {
		wanted[strings.ToLower(status)] = struct{}{}
	}
	var current *Backup
	err := c.waitUntil(ctx, 10*time.Minute, func(ctx context.Context) (bool, error) {
		backup, err := c.GetBackup(ctx, projectID, availabilityZone, backupID)
		if err != nil {
			return false, err
		}
		current = backup
		_, ok := wanted[strings.ToLower(backup.Status)]
		return ok, nil
	})
	return current, err
}
