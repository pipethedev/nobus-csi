package nobusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Flavor struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	VCPUs int    `json:"vcpus"`
	RAM   int    `json:"ram"`
	Disk  int    `json:"disk"`
}

type Image struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Project struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	AvailabilityZone string `json:"availability_zone"`
	IsDefault        bool   `json:"is_default"`
}

func (c *Client) ListFlavors(ctx context.Context, projectID, availabilityZone string) ([]Flavor, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/flavor/", projectQuery(projectID, availabilityZone), nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Flavor](resp.Data)
}

func (c *Client) FindFlavor(ctx context.Context, projectID, availabilityZone, nameOrID string) (*Flavor, error) {
	items, err := c.ListFlavors(ctx, projectID, availabilityZone)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == nameOrID || item.Name == nameOrID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("flavor %s: %w", nameOrID, ErrNotFound)
}

func (c *Client) ListImages(ctx context.Context, projectID, availabilityZone string) ([]Image, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/image/", projectQuery(projectID, availabilityZone), nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Image](resp.Data)
}

func (c *Client) FindImage(ctx context.Context, projectID, availabilityZone, nameOrID string) (*Image, error) {
	items, err := c.ListImages(ctx, projectID, availabilityZone)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == nameOrID || item.Name == nameOrID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("image %s: %w", nameOrID, ErrNotFound)
}

func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/project/", nil, nil, &resp); err != nil {
		return nil, err
	}
	return decodeList[Project](resp.Data)
}

func (c *Client) FindProject(ctx context.Context, nameOrID string) (*Project, error) {
	items, err := c.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == nameOrID || item.Name == nameOrID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("project %s: %w", nameOrID, ErrNotFound)
}
