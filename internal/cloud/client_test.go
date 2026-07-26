package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestClient_CreateVolume_UsesDocumentedEndpointAndAuth(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != volumePath {
			t.Fatalf("expected path %s, got %s", volumePath, r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("expected bearer auth, got %q", r.Header.Get("Authorization"))
		}
		var body volumeCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Size != 2 || body.ProjectID != "project" || body.AvailabilityZone != "az1" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if body.Multiattach {
			t.Fatalf("expected multiattach=false")
		}
		return jsonOK(t, &volumeResponse{
			Status: true,
			Data: apiVolume{
				ID:               "vol-1",
				Name:             "data",
				Size:             2,
				Status:           VolumeStatusAvailable,
				AvailabilityZone: "az1",
			},
		}), nil
	})
	client := newTestClient(t, transport)
	volume, err := client.CreateVolume(context.Background(), VolumeSpec{
		Name:             "data",
		SizeBytes:        2 * bytesPerGiB,
		ProjectID:        "project",
		AvailabilityZone: "az1",
	})
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if volume.ID != "vol-1" || volume.SizeBytes != 2*bytesPerGiB {
		t.Fatalf("unexpected volume: %+v", volume)
	}
}

func TestClient_GetVolumeByName_MissingReturnsNotFound(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != volumeListPath {
			t.Fatalf("expected path %s, got %s", volumeListPath, r.URL.Path)
		}
		if r.URL.Query().Get("name") != "missing" {
			t.Fatalf("expected name query")
		}
		if r.URL.Query().Get("availability_zone") != "az2" {
			t.Fatalf("expected az2 query, got %q", r.URL.Query().Get("availability_zone"))
		}
		return jsonOK(t, &volumeListResponse{Status: true}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.GetVolumeByName(context.Background(), "missing", "az2")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_AttachVolume_ReturnsDeviceFromAttachment(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if r.URL.Path != volumeAttachPath {
				t.Fatalf("expected path %s, got %s", volumeAttachPath, r.URL.Path)
			}
			var body attachVolumeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode attach request: %v", err)
			}
			if body.AvailabilityZone != "az2" {
				t.Fatalf("expected az2 attach zone, got %q", body.AvailabilityZone)
			}
			return jsonOK(t, &genericVolumeResponse{
				Status: true,
				Data:   json.RawMessage(`{}`),
			}), nil
		case 2:
			if r.URL.Path != volumePath {
				t.Fatalf("expected path %s, got %s", volumePath, r.URL.Path)
			}
			if r.URL.Query().Get("availability_zone") != "az2" {
				t.Fatalf("expected az2 lookup zone, got %q", r.URL.Query().Get("availability_zone"))
			}
			return jsonOK(t, &genericVolumeResponse{
				Status: true,
				Data: rawVolumeJSON(t, apiVolume{
					ID: "vol-1",
					Attachments: []apiAttachment{
						{ServerID: "server-1", Device: "/dev/vdb"},
					},
				}),
			}), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})
	client := newTestClient(t, transport)
	device, err := client.AttachVolume(context.Background(), "vol-1", "server-1", "az2")
	if err != nil {
		t.Fatalf("attach volume: %v", err)
	}
	if device != "/dev/vdb" {
		t.Fatalf("expected /dev/vdb, got %q", device)
	}
}

func TestClient_AttachVolume_UsesAttachResponseDevice(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != volumeAttachPath {
			t.Fatalf("expected path %s, got %s", volumeAttachPath, r.URL.Path)
		}
		return jsonOK(t, &genericVolumeResponse{
			Status: true,
			Data: rawVolumeJSON(t, apiVolume{
				ID: "vol-1",
				Attachments: []apiAttachment{
					{ServerID: "server-1", Device: "/dev/vdc"},
				},
			}),
		}), nil
	})
	client := newTestClient(t, transport)
	device, err := client.AttachVolume(context.Background(), "vol-1", "server-1", "az1")
	if err != nil {
		t.Fatalf("attach volume: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one attach request, got %d", requests)
	}
	if device != "/dev/vdc" {
		t.Fatalf("expected /dev/vdc, got %q", device)
	}
}

func TestClient_AttachVolume_PostAttachLookupFailure_ReturnsError(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonOK(t, &genericVolumeResponse{
				Status: true,
				Data:   json.RawMessage(`{}`),
			}), nil
		}
		return jsonErrorResponse(t, http.StatusBadRequest, errorResponse{Message: "volume not found"}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.AttachVolume(context.Background(), "vol-1", "server-1", "az1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_DetachVolume_NotAttachedMessageIsNotFound(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != volumeDetachPath {
			t.Fatalf("expected path %s, got %s", volumeDetachPath, r.URL.Path)
		}
		return jsonErrorResponse(t, http.StatusBadRequest, errorResponse{Message: "volume is not attached"}), nil
	})
	client := newTestClient(t, transport)
	err := client.DetachVolume(context.Background(), "vol-1", "server-1", "az1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_ResizeVolume_ReturnsProviderConfirmedSize(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if r.URL.Path != volumeExtendPath {
				t.Fatalf("expected path %s, got %s", volumeExtendPath, r.URL.Path)
			}
			var body extendVolumeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode extend request: %v", err)
			}
			if body.AvailabilityZone != "az2" {
				t.Fatalf("expected az2 extend zone, got %q", body.AvailabilityZone)
			}
			return jsonOK(t, &genericVolumeResponse{
				Status: true,
				Data:   json.RawMessage(`{}`),
			}), nil
		case 2:
			if r.URL.Path != volumePath {
				t.Fatalf("expected path %s, got %s", volumePath, r.URL.Path)
			}
			if r.URL.Query().Get("availability_zone") != "az2" {
				t.Fatalf("expected az2 lookup zone, got %q", r.URL.Query().Get("availability_zone"))
			}
			return jsonOK(t, &genericVolumeResponse{
				Status: true,
				Data: rawVolumeJSON(t, apiVolume{
					ID:   "vol-1",
					Size: 3,
				}),
			}), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})
	client := newTestClient(t, transport)
	size, err := client.ResizeVolume(context.Background(), "vol-1", 2*bytesPerGiB, "az2")
	if err != nil {
		t.Fatalf("resize volume: %v", err)
	}
	if size != 3*bytesPerGiB {
		t.Fatalf("expected provider-confirmed size %d, got %d", 3*bytesPerGiB, size)
	}
}

func TestClient_CreateSnapshot_SendsForceAndZone(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != snapshotPath {
			t.Fatalf("expected path %s, got %s", snapshotPath, r.URL.Path)
		}
		var body snapshotRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode snapshot request: %v", err)
		}
		if !body.Force {
			t.Fatalf("expected force=true")
		}
		if body.AvailabilityZone != "az2" {
			t.Fatalf("expected az2 snapshot zone, got %q", body.AvailabilityZone)
		}
		return jsonOK(t, &snapshotResponse{
			Status: true,
			Data: apiSnapshot{
				ID:       "snap-1",
				Name:     "backup",
				VolumeID: "vol-1",
				Size:     2,
			},
		}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.CreateSnapshot(context.Background(), SnapshotSpec{
		Name:             "backup",
		VolumeID:         "vol-1",
		ProjectID:        "project",
		AvailabilityZone: "az2",
		Force:            true,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
}

func TestClient_ErrorStatus_MapsRateLimit(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonErrorResponse(t, http.StatusTooManyRequests, errorResponse{Message: "slow down"}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.GetVolumeByID(context.Background(), "vol-1")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestClient_BadRequestMessage_MapsNotFound(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonErrorResponse(t, http.StatusBadRequest, errorResponse{Message: "volume not found"}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.GetVolumeByID(context.Background(), "vol-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_BadRequestMessage_MapsAlreadyExists(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonErrorResponse(t, http.StatusBadRequest, errorResponse{Message: "volume already exists"}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.GetVolumeByID(context.Background(), "vol-1")
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestClient_BadRequestMessage_DefaultsToInvalid(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonErrorResponse(t, http.StatusBadRequest, errorResponse{Message: "bad payload"}), nil
	})
	client := newTestClient(t, transport)
	_, err := client.GetVolumeByID(context.Background(), "vol-1")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestClient_ContextCanceled_DoesNotSendRequest(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		t.Fatalf("expected canceled context")
		return nil, nil
	})
	client := newTestClient(t, transport)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetVolumeByID(ctx, "vol-1")
	if err == nil {
		t.Fatalf("expected context error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newTestClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		BaseURL:          "https://cloud-api.nobus.io",
		Token:            "token",
		ProjectID:        "project",
		AvailabilityZone: "az1",
		HTTPClient:       &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

func jsonOK(t *testing.T, value responsePayload) *http.Response {
	t.Helper()
	return responseWithBody(t, http.StatusOK, value)
}

func jsonErrorResponse(t *testing.T, status int, value errorResponse) *http.Response {
	t.Helper()
	return responseWithBody(t, status, value)
}

func responseWithBody(t *testing.T, status int, value responsePayload) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(data)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func rawVolumeJSON(t *testing.T, value apiVolume) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal raw json: %v", err)
	}
	return data
}

type responsePayload interface {
	responsePayload()
}

func (*volumeResponse) responsePayload()        {}
func (*volumeListResponse) responsePayload()    {}
func (*genericVolumeResponse) responsePayload() {}
func (*snapshotResponse) responsePayload()      {}
func (errorResponse) responsePayload()          {}
