package provider

import (
	"context"
	"testing"
	"time"

	"github.com/brimble/pulumi-nobus/provider/internal/testapi"
	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func testEnv(t *testing.T) (context.Context, *testapi.Mock, func()) {
	t.Helper()
	mock := testapi.New(t)
	srv := mock.Server()
	client, err := nobusapi.NewClient(nobusapi.Config{
		Endpoint: srv.URL,
		APIKey:   "test-key",
		Poll:     time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		APIKey:           "test-key",
		Endpoint:         srv.URL,
		ProjectID:        "proj-1",
		AvailabilityZone: "ng1",
	}
	cfg.SetClientForTest(client)
	ctx := ContextWithConfig(context.Background(), cfg)
	return ctx, mock, srv.Close
}

func props(kv map[string]any) resource.PropertyMap {
	out := resource.PropertyMap{}
	for k, v := range kv {
		out[resource.PropertyKey(k)] = prop(v)
	}
	return out
}

func prop(v any) resource.PropertyValue {
	switch x := v.(type) {
	case nil:
		return resource.NewNullProperty()
	case string:
		return resource.NewStringProperty(x)
	case bool:
		return resource.NewBoolProperty(x)
	case int:
		return resource.NewNumberProperty(float64(x))
	case float64:
		return resource.NewNumberProperty(x)
	case []string:
		arr := make([]resource.PropertyValue, len(x))
		for i, s := range x {
			arr[i] = resource.NewStringProperty(s)
		}
		return resource.NewArrayProperty(arr)
	case map[string]string:
		m := resource.PropertyMap{}
		for k, s := range x {
			m[resource.PropertyKey(k)] = resource.NewStringProperty(s)
		}
		return resource.NewObjectProperty(m)
	default:
		return resource.NewStringProperty("")
	}
}

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }
