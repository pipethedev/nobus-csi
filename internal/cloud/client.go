package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/brimble/nobus-csi/internal/metadata"
)

const (
	volumePath       = "/api/v2/volume/"
	volumeListPath   = "/api/v2/volume/list"
	volumeAttachPath = "/api/v2/volume/attach"
	volumeDetachPath = "/api/v2/volume/detach"
	volumeExtendPath = "/api/v2/volume/extend"
	snapshotPath     = "/api/v2/volume/snapshot"
	snapshotListPath = "/api/v2/volume/snapshot/list"
	bytesPerGiB      = int64(1 << 30)
	defaultTimeout   = 30 * time.Second
)

type Client struct {
	base   *url.URL
	token  string
	http   *http.Client
	config ClientConfig
}

type ClientConfig struct {
	BaseURL          string
	Token            string
	ProjectID        string
	AvailabilityZone string
	HTTPClient       *http.Client
}

func NewClient(config ClientConfig) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse Nobus API URL: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("nobus API URL must be absolute")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("nobus token is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{base: base, token: config.Token, http: client, config: config}, nil
}

func (c *Client) CreateVolume(ctx context.Context, spec VolumeSpec) (*Volume, error) {
	var response volumeResponse
	payload, err := encodeBody(volumeCreateRequest{
		Size:             bytesToGiB(spec.SizeBytes),
		Name:             spec.Name,
		VolumeType:       spec.Type,
		AvailabilityZone: spec.AvailabilityZone,
		SnapshotID:       spec.SnapshotID,
		Metadata:         spec.Metadata,
		ProjectID:        spec.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	err = c.do(ctx, http.MethodPost, volumePath, nil, payload, decodeJSON(&response))
	if err != nil {
		return nil, err
	}
	return response.Data.toDomain(), nil
}

func (c *Client) GetVolumeByID(ctx context.Context, id string) (*Volume, error) {
	query := c.baseQuery()
	query.Set("volume_id", id)
	var response genericVolumeResponse
	if err := c.do(ctx, http.MethodGet, volumePath, query, nil, decodeJSON(&response)); err != nil {
		return nil, err
	}
	volume, err := decodeVolume(response.Data)
	if err != nil {
		return nil, err
	}
	return volume.toDomain(), nil
}

func (c *Client) GetVolumeByName(ctx context.Context, name string) (*Volume, error) {
	query := c.baseQuery()
	query.Set("name", name)
	var response volumeListResponse
	if err := c.do(ctx, http.MethodGet, volumeListPath, query, nil, decodeJSON(&response)); err != nil {
		return nil, err
	}
	for _, volume := range response.Data.Items {
		if volume.Name == name {
			return volume.toDomain(), nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) ListVolumes(ctx context.Context, _ Page) ([]Volume, string, error) {
	var response volumeListResponse
	if err := c.do(ctx, http.MethodGet, volumeListPath, c.baseQuery(), nil, decodeJSON(&response)); err != nil {
		return nil, "", err
	}
	volumes := make([]Volume, 0, len(response.Data.Items))
	for _, volume := range response.Data.Items {
		volumes = append(volumes, *volume.toDomain())
	}
	return volumes, "", nil
}

func (c *Client) DeleteVolume(ctx context.Context, id string) error {
	query := c.baseQuery()
	query.Set("volume_id", id)
	return c.do(ctx, http.MethodDelete, volumePath, query, nil, nil)
}

func (c *Client) ResizeVolume(ctx context.Context, id string, sizeBytes int64) (int64, error) {
	size := bytesToGiB(sizeBytes)
	payload, err := encodeBody(extendVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: c.config.AvailabilityZone,
		VolumeID:         id,
		NewSize:          size,
	})
	if err != nil {
		return 0, err
	}
	err = c.do(ctx, http.MethodPost, volumeExtendPath, nil, payload, nil)
	if err != nil {
		return 0, err
	}
	return int64(size) * bytesPerGiB, nil
}

func (c *Client) AttachVolume(ctx context.Context, volumeID, instanceID string) (string, error) {
	var response genericVolumeResponse
	payload, err := encodeBody(attachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: c.config.AvailabilityZone,
		ServerID:         instanceID,
		VolumeID:         volumeID,
	})
	if err != nil {
		return "", err
	}
	err = c.do(ctx, http.MethodPost, volumeAttachPath, nil, payload, decodeJSON(&response))
	if err != nil {
		return "", err
	}
	volume, err := decodeVolume(response.Data)
	if err != nil {
		return "", err
	}
	for _, attachment := range volume.Attachments {
		if attachment.ServerID == instanceID {
			return attachment.Device, nil
		}
	}
	return "", nil
}

func (c *Client) DetachVolume(ctx context.Context, volumeID, instanceID string) error {
	payload, err := encodeBody(detachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: c.config.AvailabilityZone,
		ServerID:         instanceID,
		VolumeID:         volumeID,
	})
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, volumeDetachPath, nil, payload, nil)
}

func (c *Client) CreateSnapshot(ctx context.Context, spec SnapshotSpec) (*Snapshot, error) {
	var response snapshotResponse
	payload, err := encodeBody(snapshotRequest{
		VolumeID:         spec.VolumeID,
		Name:             spec.Name,
		ProjectID:        spec.ProjectID,
		AvailabilityZone: spec.AvailabilityZone,
		Metadata:         spec.Metadata,
	})
	if err != nil {
		return nil, err
	}
	err = c.do(ctx, http.MethodPost, snapshotPath, nil, payload, decodeJSON(&response))
	if err != nil {
		return nil, err
	}
	return response.Data.toDomain(), nil
}

func (c *Client) DeleteSnapshot(ctx context.Context, id string) error {
	query := c.baseQuery()
	query.Set("snapshot_id", id)
	return c.do(ctx, http.MethodDelete, snapshotPath, query, nil, nil)
}

func (c *Client) ListSnapshots(ctx context.Context, _ Page) ([]Snapshot, string, error) {
	var response snapshotListResponse
	if err := c.do(ctx, http.MethodGet, snapshotListPath, c.baseQuery(), nil, decodeJSON(&response)); err != nil {
		return nil, "", err
	}
	snapshots := make([]Snapshot, 0, len(response.Data.Items))
	for _, snapshot := range response.Data.Items {
		snapshots = append(snapshots, *snapshot.toDomain())
	}
	return snapshots, "", nil
}

func (c *Client) GetInstanceMetadata(ctx context.Context) (*InstanceMetadata, error) {
	instance, err := metadata.ReadCloudInit(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Nobus instance metadata: %w: %w", err, ErrUnavailable)
	}
	return &InstanceMetadata{
		InstanceID:       instance.ID,
		AvailabilityZone: instance.AvailabilityZone,
		Region:           instance.Region,
	}, nil
}

func (c *Client) do(ctx context.Context, method string, path string, query url.Values, body []byte, decode func(io.Reader) error) error {
	endpoint := c.base.ResolveReference(&url.URL{Path: path})
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Nobus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Nobus API: %w", ErrUnavailable)
	}
	defer closeBody(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return mapHTTPError(resp)
	}
	if decode == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			return fmt.Errorf("drain Nobus response: %w", err)
		}
		return nil
	}
	return decode(resp.Body)
}

func closeBody(body io.Closer) {
	_ = body.Close()
}

func (c *Client) baseQuery() url.Values {
	query := url.Values{}
	query.Set("project_id", c.config.ProjectID)
	query.Set("availability_zone", c.config.AvailabilityZone)
	return query
}

func encodeBody(body requestBody) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(body); err != nil {
		return nil, fmt.Errorf("encode Nobus request: %w", err)
	}
	return buffer.Bytes(), nil
}

func decodeJSON(target responseBody) func(io.Reader) error {
	return func(source io.Reader) error {
		if err := json.NewDecoder(source).Decode(target); err != nil {
			return fmt.Errorf("decode Nobus response: %w", err)
		}
		return nil
	}
}

func mapHTTPError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Nobus error response: %w", ErrUnavailable)
	}
	var apiErr errorResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &apiErr)
	}
	message := apiErr.Message
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", message, ErrNotFound)
	case http.StatusConflict:
		return fmt.Errorf("%s: %w", message, ErrConflict)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: %w", message, ErrRateLimited)
	case http.StatusRequestEntityTooLarge, http.StatusForbidden:
		return fmt.Errorf("%s: %w", message, ErrQuotaExceeded)
	default:
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("%s: %w", message, ErrUnavailable)
		}
		return fmt.Errorf("%s: %w", message, ErrConflict)
	}
}

func bytesToGiB(sizeBytes int64) int {
	return int((sizeBytes + bytesPerGiB - 1) / bytesPerGiB)
}

func decodeVolume(data json.RawMessage) (*apiVolume, error) {
	var volume apiVolume
	if err := json.Unmarshal(data, &volume); err == nil && volume.ID != "" {
		return &volume, nil
	}
	var wrapped struct {
		Volume apiVolume `json:"volume"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode Nobus volume payload: %w", err)
	}
	if wrapped.Volume.ID == "" {
		return nil, ErrNotFound
	}
	return &wrapped.Volume, nil
}

type volumeCreateRequest struct {
	Size             int               `json:"size"`
	Name             string            `json:"name,omitempty"`
	VolumeType       string            `json:"volume_type,omitempty"`
	AvailabilityZone string            `json:"availability_zone"`
	SnapshotID       string            `json:"snapshot_id,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ProjectID        string            `json:"project_id"`
}

func (volumeCreateRequest) request() {}

type attachVolumeRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
	VolumeID         string `json:"volume_id"`
}

func (attachVolumeRequest) request() {}

type detachVolumeRequest = attachVolumeRequest

type extendVolumeRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	VolumeID         string `json:"volume_id"`
	NewSize          int    `json:"new_size"`
}

func (extendVolumeRequest) request() {}

type snapshotRequest struct {
	VolumeID         string            `json:"volume_id"`
	Name             string            `json:"name,omitempty"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func (snapshotRequest) request() {}

type requestBody interface {
	request()
}

type responseBody interface {
	response()
}

type volumeResponse struct {
	Status bool      `json:"status"`
	Data   apiVolume `json:"data"`
}

func (*volumeResponse) response() {}

type genericVolumeResponse struct {
	Status bool            `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func (*genericVolumeResponse) response() {}

type volumeListResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Items []apiVolume `json:"items"`
	} `json:"data"`
}

func (*volumeListResponse) response() {}

type snapshotResponse struct {
	Status bool        `json:"status"`
	Data   apiSnapshot `json:"data"`
}

func (*snapshotResponse) response() {}

type snapshotListResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Items []apiSnapshot `json:"items"`
	} `json:"data"`
}

func (*snapshotListResponse) response() {}

type errorResponse struct {
	Message string `json:"message"`
}

type apiVolume struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Size             int               `json:"size"`
	Status           VolumeStatus      `json:"status"`
	AvailabilityZone string            `json:"availability_zone"`
	VolumeType       string            `json:"volume_type"`
	Metadata         map[string]string `json:"metadata"`
	Attachments      []apiAttachment   `json:"attachments"`
}

func (v apiVolume) toDomain() *Volume {
	attachments := make([]Attachment, 0, len(v.Attachments))
	for _, attachment := range v.Attachments {
		attachments = append(attachments, Attachment{
			InstanceID: attachment.ServerID,
			DevicePath: attachment.Device,
		})
	}
	return &Volume{
		ID:               v.ID,
		Name:             v.Name,
		SizeBytes:        int64(v.Size) * bytesPerGiB,
		Status:           v.Status,
		AvailabilityZone: v.AvailabilityZone,
		Type:             v.VolumeType,
		Metadata:         v.Metadata,
		Attachments:      attachments,
	}
}

type apiAttachment struct {
	ServerID string `json:"server_id"`
	Device   string `json:"device"`
}

type apiSnapshot struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	VolumeID string            `json:"volume_id"`
	Status   SnapshotStatus    `json:"status"`
	Size     int               `json:"size"`
	Metadata map[string]string `json:"metadata"`
}

func (s apiSnapshot) toDomain() *Snapshot {
	return &Snapshot{
		ID:        s.ID,
		Name:      s.Name,
		VolumeID:  s.VolumeID,
		SizeBytes: int64(s.Size) * bytesPerGiB,
		Status:    s.Status,
		Metadata:  s.Metadata,
	}
}
