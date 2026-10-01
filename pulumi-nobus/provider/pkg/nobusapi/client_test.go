package nobusapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

func TestClientAuthAndCreateKeyPair(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody nobusapi.CreateKeyPairRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if err := json.NewEncoder(w).Encode(struct {
			Status bool             `json:"status"`
			Data   nobusapi.KeyPair `json:"data"`
		}{
			Status: true,
			Data: nobusapi.KeyPair{
				ID:          "kp-1",
				Name:        "demo",
				PublicKey:   "ssh-rsa AAA",
				PrivateKey:  "BEGIN",
				Fingerprint: "ab:cd",
			},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := nobusapi.NewClient(nobusapi.Config{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}

	kp, err := client.CreateKeyPair(context.Background(), nobusapi.CreateKeyPairRequest{
		Name:             "demo",
		ProjectID:        "proj",
		AvailabilityZone: "ng1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotPath != "/api/v3/keypair/" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Name != "demo" || gotBody.ProjectID != "proj" {
		t.Fatalf("body = %#v", gotBody)
	}
	if kp.ID != "kp-1" || kp.PrivateKey != "BEGIN" {
		t.Fatalf("keypair = %#v", kp)
	}
}

func TestGetVolumeListFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/volume/list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(struct {
			Status bool `json:"status"`
			Data   struct {
				Items []nobusapi.Volume `json:"items"`
			} `json:"data"`
		}{
			Status: true,
			Data: struct {
				Items []nobusapi.Volume `json:"items"`
			}{
				Items: []nobusapi.Volume{
					{ID: "vol-a", Name: "a", Size: 10, Status: "available"},
					{ID: "vol-b", Name: "b", Size: 20, Status: "in-use"},
				},
			},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := nobusapi.NewClient(nobusapi.Config{Endpoint: server.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	volume, err := client.GetVolume(context.Background(), "proj", "ng1", "vol-b")
	if err != nil {
		t.Fatal(err)
	}
	if volume.ID != "vol-b" || volume.Size != 20 {
		t.Fatalf("volume = %#v", volume)
	}
}

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := nobusapi.NewClient(nobusapi.Config{Endpoint: "https://cloud-api.nobus.io"})
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected api key error, got %v", err)
	}
}

func TestLiveKeyPairSmoke(t *testing.T) {
	if os.Getenv("NOBUS_LIVE") != "1" {
		t.Skip("set NOBUS_LIVE=1 to run")
	}
	apiKey := os.Getenv("NOBUS_API_KEY")
	projectID := os.Getenv("NOBUS_PROJECT_ID")
	az := os.Getenv("NOBUS_AVAILABILITY_ZONE")
	if apiKey == "" || projectID == "" || az == "" {
		t.Fatal("NOBUS_API_KEY, NOBUS_PROJECT_ID, and NOBUS_AVAILABILITY_ZONE are required")
	}
	client, err := nobusapi.NewClient(nobusapi.Config{APIKey: apiKey})
	if err != nil {
		t.Fatal(err)
	}
	name := "pulumi-provider-smoke"
	created, err := client.CreateKeyPair(context.Background(), nobusapi.CreateKeyPairRequest{
		Name:             name,
		ProjectID:        projectID,
		AvailabilityZone: az,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := created.ID
	if id == "" {
		id = created.Name
	}
	t.Cleanup(func() {
		if err := client.DeleteKeyPair(context.Background(), projectID, az, id); err != nil {
			t.Errorf("delete keypair: %v", err)
		}
	})
	got, err := client.GetKeyPair(context.Background(), projectID, az, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name && got.ID != id {
		t.Fatalf("unexpected keypair %#v", got)
	}
}
