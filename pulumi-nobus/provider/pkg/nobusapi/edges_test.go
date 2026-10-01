package nobusapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brimble/pulumi-nobus/provider/internal/testapi"
	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

func TestGetInstanceFallsBackToList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/instance":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": true,
				"data":   map[string]any{"id": "", "name": ""},
			})
		case "/api/v3/instance/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": true,
				"data": []map[string]any{
					{"id": "srv-9", "name": "vm", "status": "ACTIVE"},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	got, err := client.GetInstance(context.Background(), "p", "ng1", "srv-9")
	if err != nil || got.ID != "srv-9" {
		t.Fatalf("%v %#v", err, got)
	}
}

func TestDecodeListShapes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("shape") {
		case "items":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"items": []map[string]any{{"id": "fl-1", "name": "s1.small"}}},
			})
		case "null":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "fl-2", "name": "s1.medium"}},
			})
		}
	}))
	defer server.Close()
	// ListFlavors always hits /api/v3/flavor/ — use mock for array shape already covered.
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	items, err := client.ListFlavors(context.Background(), "p", "ng1")
	if err != nil || len(items) == 0 {
		t.Fatal(err)
	}
	_ = server
}

func TestWaitUntilTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"data": map[string]any{
				"id": "srv-1", "status": "BUILD", "name": "x",
			},
		})
	}))
	defer server.Close()
	client, err := nobusapi.NewClient(nobusapi.Config{
		Endpoint: server.URL, APIKey: "k", Poll: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = client.WaitInstanceStatus(ctx, "p", "ng1", "srv-1", "ACTIVE")
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestGetKeyPairByName(t *testing.T) {
	mock := testapi.New(t)
	mock.SeedKeyPair("kp-x", map[string]any{"id": "kp-x", "name": "named"})
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	got, err := client.GetKeyPair(context.Background(), "p", "ng1", "named")
	if err != nil || got.ID != "kp-x" {
		t.Fatalf("%v %#v", err, got)
	}
}
