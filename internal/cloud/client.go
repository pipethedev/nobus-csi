package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	loginPath        = "/api/v2/auth/login"
	bytesPerGiB      = int64(1 << 30)
	defaultTimeout   = 30 * time.Second
)

type Client struct {
	base   *url.URL
	mu     sync.Mutex
	token  string
	http   *http.Client
	config ClientConfig
}

type ClientConfig struct {
	BaseURL          string
	Token            string
	Email            string
	Password         string
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
	if config.Token == "" && (config.Email == "" || config.Password == "") {
		return nil, fmt.Errorf("nobus token or email and password are required")
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
	volume := response.Data.toDomain()
	volume.AvailabilityZone = spec.AvailabilityZone
	return volume, nil
}

func (c *Client) GetVolumeByID(ctx context.Context, id string) (*Volume, error) {
	return c.getVolumeByIDInZone(ctx, id, "")
}

func (c *Client) getVolumeByIDInZone(ctx context.Context, id string, availabilityZone string) (*Volume, error) {
	zone := firstValue(availabilityZone, c.config.AvailabilityZone)
	volumes, _, err := c.ListVolumes(ctx, Page{ID: id, AvailabilityZone: zone})
	if err != nil {
		return nil, err
	}
	for _, volume := range volumes {
		if volume.ID == id {
			clone := volume
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) GetVolumeByName(ctx context.Context, name string, availabilityZone string) (*Volume, error) {
	volumes, _, err := c.ListVolumes(ctx, Page{Name: name, AvailabilityZone: availabilityZone})
	if err != nil {
		return nil, err
	}
	for _, volume := range volumes {
		if volume.Name == name {
			clone := volume
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) ListVolumes(ctx context.Context, page Page) ([]Volume, string, error) {
	query := c.zoneQuery(firstValue(page.AvailabilityZone, c.config.AvailabilityZone))
	if page.ID != "" {
		query.Set("id", page.ID)
	}
	if page.Name != "" {
		query.Set("name", page.Name)
	}
	var response volumeListResponse
	if err := c.do(ctx, http.MethodGet, volumeListPath, query, nil, decodeJSON(&response)); err != nil {
		return nil, "", err
	}
	volumes := make([]Volume, 0, len(response.Data.Items))
	zone := query.Get("availability_zone")
	for _, volume := range response.Data.Items {
		domain := volume.toDomain()
		if zone != "" {
			domain.AvailabilityZone = zone
		}
		volumes = append(volumes, *domain)
	}
	paged, token, err := paginate(volumes, page)
	if err != nil {
		return nil, "", err
	}
	return paged, token, nil
}

func (c *Client) DeleteVolume(ctx context.Context, id string) error {
	query := c.zoneQuery(c.config.AvailabilityZone)
	query.Set("volume_id", id)
	if err := c.do(ctx, http.MethodDelete, volumePath, query, nil, nil); err != nil {
		return err
	}
	if _, err := c.getVolumeByIDInZone(ctx, id, c.config.AvailabilityZone); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	return ErrUnavailable
}

func (c *Client) ResizeVolume(ctx context.Context, id string, sizeBytes int64, availabilityZone string) (int64, error) {
	zone := firstValue(availabilityZone, c.config.AvailabilityZone)
	size := bytesToGiB(sizeBytes)
	payload, err := encodeBody(extendVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: zone,
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
	volume, err := c.getVolumeByIDInZone(ctx, id, zone)
	if err != nil {
		return 0, err
	}
	return volume.SizeBytes, nil
}

func (c *Client) AttachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) (string, error) {
	zone := firstValue(availabilityZone, c.config.AvailabilityZone)
	var response genericVolumeResponse
	payload, err := encodeBody(attachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: zone,
		ServerID:         instanceID,
		VolumeID:         volumeID,
	})
	if err != nil {
		return "", err
	}
	if err := c.do(ctx, http.MethodPost, volumeAttachPath, nil, payload, decodeJSON(&response)); err != nil {
		return "", err
	}
	if device, ok := attachedDeviceFromData(response.Data, instanceID); ok {
		return device, nil
	}
	volume, err := c.getVolumeByIDInZone(ctx, volumeID, zone)
	if err != nil {
		return "", err
	}
	for _, attachment := range volume.Attachments {
		if attachment.InstanceID == instanceID {
			return attachment.DevicePath, nil
		}
	}
	return "", fmt.Errorf("attached volume %s has no device for instance %s: %w", volumeID, instanceID, ErrUnavailable)
}

func (c *Client) DetachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) error {
	payload, err := encodeBody(detachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: firstValue(availabilityZone, c.config.AvailabilityZone),
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
		Force:            spec.Force,
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

func (c *Client) DeleteSnapshot(ctx context.Context, id string, availabilityZone string) error {
	query := c.zoneQuery(availabilityZone)
	query.Set("snapshot_id", id)
	return c.do(ctx, http.MethodDelete, snapshotPath, query, nil, nil)
}

func (c *Client) ListSnapshots(ctx context.Context, page Page) ([]Snapshot, string, error) {
	query := c.zoneQuery(firstValue(page.AvailabilityZone, c.config.AvailabilityZone))
	if page.Name != "" {
		query.Set("name", page.Name)
	}
	var response snapshotListResponse
	if err := c.do(ctx, http.MethodGet, snapshotListPath, query, nil, decodeJSON(&response)); err != nil {
		return nil, "", err
	}
	snapshots := make([]Snapshot, 0, len(response.Data.Items))
	for _, snapshot := range response.Data.Items {
		domain := snapshot.toDomain()
		if domain.AvailabilityZone == "" {
			domain.AvailabilityZone = query.Get("availability_zone")
		}
		snapshots = append(snapshots, *domain)
	}
	paged, token, err := paginate(snapshots, page)
	if err != nil {
		return nil, "", err
	}
	return paged, token, nil
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
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.bearerToken(ctx)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create Nobus request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("call Nobus API: %w", ErrUnavailable)
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 && c.canLogin() {
			_, _ = io.Copy(io.Discard, resp.Body)
			closeBody(resp.Body)
			c.clearToken(token)
			continue
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
		err = decode(resp.Body)
		return err
	}
	return fmt.Errorf("refresh Nobus token: %w", ErrUnauthorized)
}

func (c *Client) bearerToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()
	token, err := c.login(ctx)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}
	c.token = token
	return token, nil
}

func (c *Client) clearToken(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == value {
		c.token = ""
	}
}

func (c *Client) canLogin() bool {
	return c.config.Email != "" && c.config.Password != ""
}

func (c *Client) login(ctx context.Context) (string, error) {
	payload, err := encodeBody(loginRequest{
		Email:    c.config.Email,
		Password: c.config.Password,
	})
	if err != nil {
		return "", err
	}
	endpoint := c.base.ResolveReference(&url.URL{Path: loginPath})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create Nobus login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Nobus login API: %w", ErrUnavailable)
	}
	defer closeBody(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", mapHTTPError(resp)
	}
	var response loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode Nobus login response: %w", err)
	}
	if response.Token == "" {
		return "", fmt.Errorf("nobus login response missing token: %w", ErrUnauthorized)
	}
	return response.Token, nil
}

func closeBody(body io.Closer) {
	_ = body.Close()
}

func (c *Client) baseQuery() url.Values {
	query := url.Values{}
	query.Set("project_id", c.config.ProjectID)
	return query
}

func (c *Client) zoneQuery(availabilityZone string) url.Values {
	query := c.baseQuery()
	query.Set("availability_zone", firstValue(availabilityZone, c.config.AvailabilityZone))
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
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s: %w", message, ErrUnauthorized)
	case http.StatusRequestEntityTooLarge:
		return fmt.Errorf("%s: %w", message, ErrQuotaExceeded)
	default:
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("%s: %w", message, ErrUnavailable)
		}
		return classifyClientError(message)
	}
}

func classifyClientError(message string) error {
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(normalized, "not found"),
		strings.Contains(normalized, "does not exist"),
		strings.Contains(normalized, "not attached"),
		strings.Contains(normalized, "already detached"):
		return fmt.Errorf("%s: %w", message, ErrNotFound)
	case strings.Contains(normalized, "already exist"), strings.Contains(normalized, "duplicate"):
		return fmt.Errorf("%s: %w", message, ErrAlreadyExists)
	case strings.Contains(normalized, "quota"):
		return fmt.Errorf("%s: %w", message, ErrQuotaExceeded)
	case strings.Contains(normalized, "rate"):
		return fmt.Errorf("%s: %w", message, ErrRateLimited)
	default:
		return fmt.Errorf("%s: %w", message, ErrInvalid)
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
		Volume apiVolume   `json:"volume"`
		Item   apiVolume   `json:"item"`
		Items  []apiVolume `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode Nobus volume payload: %w", err)
	}
	if wrapped.Volume.ID != "" {
		return &wrapped.Volume, nil
	}
	if wrapped.Item.ID != "" {
		return &wrapped.Item, nil
	}
	if len(wrapped.Items) > 0 && wrapped.Items[0].ID != "" {
		return &wrapped.Items[0], nil
	}
	return nil, ErrNotFound
}

func attachedDeviceFromData(data json.RawMessage, instanceID string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	volume, err := decodeVolume(data)
	if err != nil {
		return "", false
	}
	for _, attachment := range volume.toDomain().Attachments {
		if attachment.InstanceID == instanceID && attachment.DevicePath != "" {
			return attachment.DevicePath, true
		}
	}
	return "", false
}

func firstValue(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func paginate[T any](items []T, page Page) ([]T, string, error) {
	start := 0
	if page.Token != "" {
		parsed, err := strconv.Atoi(page.Token)
		if err != nil || parsed < 0 {
			return nil, "", fmt.Errorf("invalid page token: %w", ErrInvalid)
		}
		start = parsed
	}
	if start >= len(items) {
		return nil, "", nil
	}
	if page.Size <= 0 || start+page.Size >= len(items) {
		return items[start:], "", nil
	}
	next := start + page.Size
	return items[start:next], strconv.Itoa(next), nil
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
	Force            bool              `json:"force"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func (snapshotRequest) request() {}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (loginRequest) request() {}

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

type loginResponse struct {
	Token string `json:"token"`
}

type apiVolume struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Size             int               `json:"size"`
	Status           VolumeStatus      `json:"status"`
	AvailabilityZone string            `json:"availability_zone"`
	VolumeType       string            `json:"volume_type"`
	Multiattach      bool              `json:"multiattach"`
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
		Multiattach:      v.Multiattach,
		Metadata:         v.Metadata,
		Attachments:      attachments,
	}
}

type apiAttachment struct {
	ServerID string `json:"server_id"`
	Device   string `json:"device"`
}

type apiSnapshot struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	VolumeID         string            `json:"volume_id"`
	Status           SnapshotStatus    `json:"status"`
	Size             int               `json:"size"`
	CreatedAt        string            `json:"created_at"`
	AvailabilityZone string            `json:"availability_zone"`
	Metadata         map[string]string `json:"metadata"`
}

func (s apiSnapshot) toDomain() *Snapshot {
	return &Snapshot{
		ID:               s.ID,
		Name:             s.Name,
		VolumeID:         s.VolumeID,
		SizeBytes:        int64(s.Size) * bytesPerGiB,
		Status:           s.Status,
		CreatedAt:        parseNobusTime(s.CreatedAt),
		AvailabilityZone: s.AvailabilityZone,
		Metadata:         s.Metadata,
	}
}

func parseNobusTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05.999", "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
