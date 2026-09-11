package cloud

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brimble/nobus-csi/internal/metadata"
)

const (
	volumePath       = "/api/v3/volume/"
	volumeListPath   = "/api/v3/volume/list"
	volumeExtendPath = "/api/v3/volume/extend"
	snapshotPath     = "/api/v3/volume/snapshot"
	snapshotListPath = "/api/v3/volume/snapshot/list"
	volumeAttachPath = "/api/v3/volume/attach"
	volumeDetachPath = "/api/v3/volume/detach"
	bytesPerGiB      = int64(1 << 30)
	defaultTimeout   = 30 * time.Second
	attachWait       = 3 * time.Minute
	attachPoll       = 2 * time.Second
)

type Client struct {
	base   *url.URL
	http   *http.Client
	config ClientConfig
	poll   time.Duration
}

type ClientConfig struct {
	BaseURL          string
	APIKey           string
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
	if config.APIKey == "" {
		return nil, fmt.Errorf("nobus API key is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{base: base, http: client, config: config, poll: attachPoll}, nil
}

func (c *Client) CreateVolume(ctx context.Context, spec VolumeSpec) (*Volume, error) {
	var response volumeResponse
	payload, err := marshalJSON(volumeCreateRequest{
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
	if err := c.do(ctx, http.MethodPost, volumePath, nil, payload, decodeJSON(&response)); err != nil {
		return nil, err
	}
	volume := response.Data.toDomain()
	volume.AvailabilityZone = spec.AvailabilityZone
	return &volume, nil
}

func (c *Client) GetVolumeByID(ctx context.Context, id string) (*Volume, error) {
	return c.getVolumeByIDInZone(ctx, id, "")
}

func (c *Client) getVolumeByIDInZone(ctx context.Context, id string, availabilityZone string) (*Volume, error) {
	query := c.zoneQuery(availabilityZone)
	query.Set("volume_id", id)
	var response volumeGetResponse
	if err := c.do(ctx, http.MethodGet, volumePath, query, nil, decodeJSON(&response)); err != nil {
		return nil, err
	}
	if response.Data == nil || response.Data.ID == "" {
		return nil, ErrNotFound
	}
	volume := response.Data.toDomain()
	volume.AvailabilityZone = query.Get("availability_zone")
	return &volume, nil
}

func (c *Client) ListVolumes(ctx context.Context, page Page) ([]Volume, string, error) {
	query := c.zoneQuery(page.AvailabilityZone)
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
		volumes = append(volumes, domain)
	}
	slices.SortFunc(volumes, func(left Volume, right Volume) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return paginate(volumes, page)
}

func (c *Client) DeleteVolume(ctx context.Context, id string) error {
	query := c.zoneQuery("")
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
	zone := cmp.Or(availabilityZone, c.config.AvailabilityZone)
	payload, err := marshalJSON(extendVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: zone,
		VolumeID:         id,
		NewSize:          bytesToGiB(sizeBytes),
	})
	if err != nil {
		return 0, err
	}
	if err := c.do(ctx, http.MethodPost, volumeExtendPath, nil, payload, nil); err != nil {
		return 0, err
	}
	volume, err := c.getVolumeByIDInZone(ctx, id, zone)
	if err != nil {
		return 0, err
	}
	return volume.SizeBytes, nil
}

func (c *Client) AttachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) (string, error) {
	payload, err := marshalJSON(attachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: cmp.Or(availabilityZone, c.config.AvailabilityZone),
		ServerID:         instanceID,
		VolumeID:         volumeID,
	})
	if err != nil {
		return "", err
	}
	if err := c.do(ctx, http.MethodPost, volumeAttachPath, nil, payload, nil); err != nil {
		return "", err
	}
	return c.waitAttachedDevice(ctx, volumeID, instanceID, cmp.Or(availabilityZone, c.config.AvailabilityZone))
}

func (c *Client) waitAttachedDevice(ctx context.Context, volumeID string, instanceID string, zone string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, attachWait)
	defer cancel()
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	for {
		volume, err := c.getVolumeByIDInZone(ctx, volumeID, zone)
		if err != nil {
			return "", fmt.Errorf("wait for attached volume %s: %w", volumeID, err)
		}
		for _, attachment := range volume.Attachments {
			if attachment.InstanceID == instanceID && attachment.DevicePath != "" {
				return attachment.DevicePath, nil
			}
		}
		timer.Reset(c.poll)
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("attached volume %s has no device for instance %s: %w", volumeID, instanceID, ErrUnavailable)
		case <-timer.C:
		}
	}
}

func (c *Client) DetachVolume(ctx context.Context, volumeID, instanceID string, availabilityZone string) error {
	payload, err := marshalJSON(attachVolumeRequest{
		ProjectID:        c.config.ProjectID,
		AvailabilityZone: cmp.Or(availabilityZone, c.config.AvailabilityZone),
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
	payload, err := marshalJSON(snapshotRequest{
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
	if err := c.do(ctx, http.MethodPost, snapshotPath, nil, payload, decodeJSON(&response)); err != nil {
		return nil, err
	}
	snapshot, err := response.Data.toDomain()
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (c *Client) DeleteSnapshot(ctx context.Context, id string, availabilityZone string) error {
	query := c.zoneQuery(availabilityZone)
	query.Set("snapshot_id", id)
	return c.do(ctx, http.MethodDelete, snapshotPath, query, nil, nil)
}

func (c *Client) ListSnapshots(ctx context.Context, page Page) ([]Snapshot, string, error) {
	query := c.zoneQuery(page.AvailabilityZone)
	if page.Name != "" {
		query.Set("name", page.Name)
	}
	var response snapshotListResponse
	if err := c.do(ctx, http.MethodGet, snapshotListPath, query, nil, decodeJSON(&response)); err != nil {
		return nil, "", err
	}
	snapshots := make([]Snapshot, 0, len(response.Data.Items))
	for _, snapshot := range response.Data.Items {
		domain, err := snapshot.toDomain()
		if err != nil {
			return nil, "", err
		}
		if domain.AvailabilityZone == "" {
			domain.AvailabilityZone = query.Get("availability_zone")
		}
		snapshots = append(snapshots, domain)
	}
	slices.SortFunc(snapshots, func(left Snapshot, right Snapshot) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return paginate(snapshots, page)
}

func (c *Client) GetInstanceMetadata(ctx context.Context) (*InstanceMetadata, error) {
	instance, err := metadata.ReadCloudInit(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
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
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Nobus API: %w", errors.Join(ErrUnavailable, err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return mapHTTPError(resp)
	}
	if decode == nil {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return fmt.Errorf("drain Nobus response: %w", err)
		}
		return nil
	}
	return decode(resp.Body)
}

func (c *Client) zoneQuery(availabilityZone string) url.Values {
	query := url.Values{}
	query.Set("project_id", c.config.ProjectID)
	query.Set("availability_zone", cmp.Or(availabilityZone, c.config.AvailabilityZone))
	return query
}

func marshalJSON(body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Nobus request: %w", err)
	}
	return data, nil
}

func decodeJSON(target any) func(io.Reader) error {
	return func(source io.Reader) error {
		if err := json.NewDecoder(source).Decode(target); err != nil {
			return fmt.Errorf("decode Nobus response: %w", errors.Join(ErrInvalid, err))
		}
		return nil
	}
}

func mapHTTPError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Nobus error response: %w", errors.Join(ErrUnavailable, err))
	}
	var apiErr errorResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &apiErr)
	}
	message := cmp.Or(apiErr.Message, http.StatusText(resp.StatusCode))
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

type attachVolumeRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	ServerID         string `json:"server_id"`
	VolumeID         string `json:"volume_id"`
}

type extendVolumeRequest struct {
	ProjectID        string `json:"project_id"`
	AvailabilityZone string `json:"availability_zone"`
	VolumeID         string `json:"volume_id"`
	NewSize          int    `json:"new_size"`
}

type snapshotRequest struct {
	VolumeID         string            `json:"volume_id"`
	Name             string            `json:"name,omitempty"`
	Force            bool              `json:"force"`
	ProjectID        string            `json:"project_id"`
	AvailabilityZone string            `json:"availability_zone"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type volumeResponse struct {
	Status bool      `json:"status"`
	Data   apiVolume `json:"data"`
}

type volumeGetResponse struct {
	Status bool       `json:"status"`
	Data   *apiVolume `json:"data"`
}

type volumeListResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Items []apiVolume `json:"items"`
	} `json:"data"`
}

type snapshotResponse struct {
	Status bool        `json:"status"`
	Data   apiSnapshot `json:"data"`
}

type snapshotListResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Items []apiSnapshot `json:"items"`
	} `json:"data"`
}

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
	Multiattach      bool              `json:"multiattach"`
	Metadata         map[string]string `json:"metadata"`
	Attachments      []apiAttachment   `json:"attachments"`
}

func (v apiVolume) toDomain() Volume {
	attachments := make([]Attachment, 0, len(v.Attachments))
	for _, attachment := range v.Attachments {
		attachments = append(attachments, Attachment{
			InstanceID: attachment.ServerID,
			DevicePath: attachment.Device,
		})
	}
	return Volume{
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

func (s apiSnapshot) toDomain() (Snapshot, error) {
	createdAt, err := parseNobusTime(s.CreatedAt)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		ID:               s.ID,
		Name:             s.Name,
		VolumeID:         s.VolumeID,
		SizeBytes:        int64(s.Size) * bytesPerGiB,
		Status:           s.Status,
		CreatedAt:        createdAt,
		AvailabilityZone: s.AvailabilityZone,
		Metadata:         s.Metadata,
	}, nil
}

func parseNobusTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05.999", "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse Nobus timestamp %q: %w", value, ErrInvalid)
}
