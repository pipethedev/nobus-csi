package nobusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type KeyPair struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PublicKey   string `json:"public_key"`
	PrivateKey  string `json:"private_key"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	UserID      string `json:"user_id"`
}

type CreateKeyPairRequest struct {
	Name             string `json:"name"`
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
}

func (c *Client) CreateKeyPair(ctx context.Context, req CreateKeyPairRequest) (*KeyPair, error) {
	var resp struct {
		Status bool    `json:"status"`
		Data   KeyPair `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/keypair/", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListKeyPairs(ctx context.Context, projectID, availabilityZone string) ([]KeyPair, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/keypair/", projectQuery(projectID, availabilityZone), nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[KeyPair](resp.Data)
}

func (c *Client) GetKeyPair(ctx context.Context, projectID, availabilityZone, nameOrID string) (*KeyPair, error) {
	items, err := c.ListKeyPairs(ctx, projectID, availabilityZone)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == nameOrID || item.Name == nameOrID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("keypair %s: %w", nameOrID, ErrNotFound)
}

func (c *Client) DeleteKeyPair(ctx context.Context, projectID, availabilityZone, keypairID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("keypair_id", keypairID)
	return c.do(ctx, http.MethodDelete, "/api/v3/keypair/", q, nil, nil)
}

type Instance struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	KeyName     string    `json:"key_name"`
	Created     string    `json:"created"`
	Updated     string    `json:"updated"`
	Addresses   Addresses `json:"addresses"`
}

type CreateInstanceRequest struct {
	ProjectID        string   `json:"project_id"`
	AvailabilityZone string   `json:"availability_zone"`
	Name             string   `json:"name"`
	Image            string   `json:"image"`
	Flavor           string   `json:"flavor"`
	KeyName          string   `json:"key_name"`
	VolumeSize       *int     `json:"volume_size,omitempty"`
	NetworkID        string   `json:"network_id,omitempty"`
	SecurityGroups   []string `json:"security_groups,omitempty"`
	AdminPass        string   `json:"admin_pass,omitempty"`
	UserData         string   `json:"userdata,omitempty"`
}

func (c *Client) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	var resp struct {
		Status bool     `json:"status"`
		Data   Instance `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/instance", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetInstance(ctx context.Context, projectID, availabilityZone, serverID string) (*Instance, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("server_id", serverID)
	var resp struct {
		Status bool     `json:"status"`
		Data   Instance `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/instance", q, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data.ID == "" {
		return c.getInstanceFromList(ctx, projectID, availabilityZone, serverID)
	}
	return &resp.Data, nil
}

func (c *Client) getInstanceFromList(ctx context.Context, projectID, availabilityZone, serverID string) (*Instance, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("id", serverID)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/instance/list", q, nil, &resp); err != nil {
		return nil, err
	}
	items, err := decodeList[Instance](resp.Data)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == serverID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("instance %s: %w", serverID, ErrNotFound)
}

func (c *Client) DeleteInstance(ctx context.Context, projectID, availabilityZone, serverID string, deleteVolume bool) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("server_id", serverID)
	if deleteVolume {
		q.Set("delete_volume", "true")
	}
	return c.do(ctx, http.MethodDelete, "/api/v3/instance", q, nil, nil)
}

type UpdateInstanceRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
}

type ResizeInstanceRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
	FlavorID         string `json:"flavor_id"`
}

type BaseServerRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
}

type SecurityGroupActionRequest struct {
	ProjectID         string `json:"project_id"`
	AvailabilityZone  string `json:"availability_zone"`
	ServerID          string `json:"server_id"`
	SecurityGroupName string `json:"security_group_name"`
}

type SetTagsRequest struct {
	ProjectID        string   `json:"project_id"`
	AvailabilityZone string   `json:"availability_zone"`
	ServerID         string   `json:"server_id"`
	Tags             []string `json:"tags"`
}

func (c *Client) UpdateInstance(ctx context.Context, req UpdateInstanceRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/update-server", nil, req, nil)
}

func (c *Client) ResizeInstance(ctx context.Context, req ResizeInstanceRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/resize", nil, req, nil)
}

func (c *Client) ConfirmResize(ctx context.Context, req BaseServerRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/confirm-resize", nil, req, nil)
}

func (c *Client) AttachSecurityGroup(ctx context.Context, req SecurityGroupActionRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/attach-security-group", nil, req, nil)
}

func (c *Client) RemoveSecurityGroup(ctx context.Context, req SecurityGroupActionRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/remove-security-group", nil, req, nil)
}

func (c *Client) SetTags(ctx context.Context, req SetTagsRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v3/instance/set-tags", nil, req, nil)
}

func (c *Client) WaitInstanceStatus(ctx context.Context, projectID, availabilityZone, serverID string, want ...string) (*Instance, error) {
	wanted := map[string]struct{}{}
	for _, status := range want {
		wanted[strings.ToLower(status)] = struct{}{}
	}
	var current *Instance
	err := c.waitUntil(ctx, 15*time.Minute, func(ctx context.Context) (bool, error) {
		inst, err := c.GetInstance(ctx, projectID, availabilityZone, serverID)
		if err != nil {
			return false, err
		}
		current = inst
		_, ok := wanted[strings.ToLower(inst.Status)]
		return ok, nil
	})
	return current, err
}
