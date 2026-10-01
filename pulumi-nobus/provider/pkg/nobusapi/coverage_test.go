package nobusapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brimble/pulumi-nobus/provider/internal/testapi"
	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

func testClient(t *testing.T, serverURL string) *nobusapi.Client {
	t.Helper()
	client, err := nobusapi.NewClient(nobusapi.Config{
		Endpoint: serverURL,
		APIKey:   "test-key",
		Poll:     time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestNewClientValidation(t *testing.T) {
	if _, err := nobusapi.NewClient(nobusapi.Config{}); err == nil {
		t.Fatal("expected api key error")
	}
	if _, err := nobusapi.NewClient(nobusapi.Config{APIKey: "k", Endpoint: "://bad"}); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := nobusapi.NewClient(nobusapi.Config{APIKey: "k", Endpoint: "/relative"}); err == nil {
		t.Fatal("expected absolute url error")
	}
	client, err := nobusapi.NewClient(nobusapi.Config{APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("nil client")
	}
}

func TestClientHTTPErrorsAndRetry(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"temporarily unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":[{"id":"fl-1","name":"s1.small","vcpus":1,"ram":1,"disk":1}]}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	flavors, err := client.ListFlavors(context.Background(), "p", "ng1")
	if err != nil {
		t.Fatal(err)
	}
	if len(flavors) != 1 || hits.Load() != 3 {
		t.Fatalf("flavors=%v hits=%d", flavors, hits.Load())
	}
}

func TestClientMapsNotFoundUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "missing"):
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"gone"}`))
		case strings.Contains(r.URL.Path, "deny"):
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		case strings.Contains(r.URL.Path, "bad"):
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"message":"invalid payload"}`))
		case strings.Contains(r.URL.Path, "ratelimit"):
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"message":"too many requests"}`))
		default:
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`not-json`))
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)

	err := client.DeleteKeyPair(context.Background(), "p", "ng1", "x")
	// Delete hits path /api/v3/keypair/ which falls to 500 after retries
	if err == nil {
		t.Fatal("expected error")
	}

	mock := testapi.New(t)
	mock.FailNext["/api/v3/keypair/"] = 404
	srv := mock.Server()
	defer srv.Close()
	c2 := testClient(t, srv.URL)
	if err := c2.DeleteKeyPair(context.Background(), "p", "ng1", "missing"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}

	mock.FailNext["/api/v3/keypair/"] = 401
	if err := c2.DeleteKeyPair(context.Background(), "p", "ng1", "x"); !errors.Is(err, nobusapi.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}

	mock.FailNext["/api/v3/keypair/"] = 400
	if err := c2.DeleteKeyPair(context.Background(), "p", "ng1", "x"); !errors.Is(err, nobusapi.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestCatalogFinders(t *testing.T) {
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	ctx := context.Background()

	fl, err := client.FindFlavor(ctx, "p", "ng1", "s1.small")
	if err != nil || fl.ID != "fl-1" {
		t.Fatalf("%v %#v", err, fl)
	}
	if _, err := client.FindFlavor(ctx, "p", "ng1", "nope"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}

	img, err := client.FindImage(ctx, "p", "ng1", "Ubuntu")
	if err != nil || img.ID != "img-1" {
		t.Fatalf("%v %#v", err, img)
	}
	if _, err := client.FindImage(ctx, "p", "ng1", "nope"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}

	proj, err := client.FindProject(ctx, "default")
	if err != nil || !proj.IsDefault {
		t.Fatalf("%v %#v", err, proj)
	}
	if _, err := client.FindProject(ctx, "missing"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestComputeLifecycle(t *testing.T) {
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	ctx := context.Background()

	kp, err := client.CreateKeyPair(ctx, nobusapi.CreateKeyPairRequest{Name: "k", ProjectID: "p", AvailabilityZone: "ng1"})
	if err != nil || kp.ID == "" {
		t.Fatal(err)
	}
	got, err := client.GetKeyPair(ctx, "p", "ng1", kp.ID)
	if err != nil || got.Name != "k" {
		t.Fatalf("%v %#v", err, got)
	}
	list, err := client.ListKeyPairs(ctx, "p", "ng1")
	if err != nil || len(list) == 0 {
		t.Fatal(err)
	}
	if err := client.DeleteKeyPair(ctx, "p", "ng1", kp.ID); err != nil {
		t.Fatal(err)
	}

	inst, err := client.CreateInstance(ctx, nobusapi.CreateInstanceRequest{
		ProjectID: "p", AvailabilityZone: "ng1", Name: "vm", Image: "img", Flavor: "fl", KeyName: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	active, err := client.WaitInstanceStatus(ctx, "p", "ng1", inst.ID, "ACTIVE")
	if err != nil || active.Status != "ACTIVE" {
		t.Fatalf("%v %#v", err, active)
	}
	if err := client.UpdateInstance(ctx, nobusapi.UpdateInstanceRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID, Name: "vm2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.ResizeInstance(ctx, nobusapi.ResizeInstanceRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID, FlavorID: "fl-2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.ConfirmResize(ctx, nobusapi.BaseServerRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.AttachSecurityGroup(ctx, nobusapi.SecurityGroupActionRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID, SecurityGroupName: "web",
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveSecurityGroup(ctx, nobusapi.SecurityGroupActionRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID, SecurityGroupName: "web",
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetTags(ctx, nobusapi.SetTagsRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: inst.ID, Tags: []string{"a"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteInstance(ctx, "p", "ng1", inst.ID, true); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeLifecycle(t *testing.T) {
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	ctx := context.Background()

	vol, err := client.CreateVolume(ctx, nobusapi.CreateVolumeRequest{
		Size: 10, Name: "v", AvailabilityZone: "ng1", ProjectID: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := client.WaitVolumeStatus(ctx, "p", "ng1", vol.ID, "available")
	if err != nil || ready.Status != "available" {
		t.Fatalf("%v %#v", err, ready)
	}
	updated, err := client.UpdateVolume(ctx, vol.ID, nobusapi.UpdateVolumeRequest{
		Name: "v2", ProjectID: "p", AvailabilityZone: "ng1",
	})
	if err != nil || updated.Name != "v2" {
		t.Fatalf("%v %#v", err, updated)
	}
	if err := client.AttachVolume(ctx, nobusapi.AttachVolumeRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: "srv-1", VolumeID: vol.ID,
	}); err != nil {
		t.Fatal(err)
	}
	device, err := client.WaitVolumeAttached(ctx, "p", "ng1", vol.ID, "srv-1")
	if err != nil || device == "" {
		t.Fatalf("%v %q", err, device)
	}
	if err := client.DetachVolume(ctx, nobusapi.AttachVolumeRequest{
		ProjectID: "p", AvailabilityZone: "ng1", ServerID: "srv-1", VolumeID: vol.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteVolume(ctx, "p", "ng1", vol.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetVolume(ctx, "p", "ng1", "missing"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestSnapshotAndBackupLifecycle(t *testing.T) {
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	ctx := context.Background()

	snap, err := client.CreateSnapshot(ctx, nobusapi.CreateSnapshotRequest{
		VolumeID: "vol-1", Name: "s", ProjectID: "p", AvailabilityZone: "ng1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := client.WaitSnapshotStatus(ctx, "p", "ng1", snap.ID, "available")
	if err != nil || ready.Status != "available" {
		t.Fatalf("%v %#v", err, ready)
	}
	got, err := client.GetSnapshot(ctx, "p", "ng1", snap.ID)
	if err != nil || got.ID != snap.ID {
		t.Fatalf("%v %#v", err, got)
	}
	listed, err := client.ListSnapshots(ctx, "p", "ng1")
	if err != nil || len(listed) == 0 {
		t.Fatalf("%v %#v", err, listed)
	}

	bak, err := client.CreateBackup(ctx, nobusapi.CreateBackupRequest{
		VolumeID: "vol-1", Name: "b", ProjectID: "p", AvailabilityZone: "ng1",
	})
	if err != nil {
		t.Fatal(err)
	}
	bReady, err := client.WaitBackupStatus(ctx, "p", "ng1", bak.ID, "available")
	if err != nil || bReady.Status != "available" {
		t.Fatalf("%v %#v", err, bReady)
	}
	bGot, err := client.GetBackup(ctx, "p", "ng1", bak.ID)
	if err != nil || bGot.ID != bak.ID {
		t.Fatalf("%v %#v", err, bGot)
	}
	bListed, err := client.ListBackups(ctx, "p", "ng1")
	if err != nil || len(bListed) == 0 {
		t.Fatalf("%v %#v", err, bListed)
	}

	zones, err := client.ListAvailabilityZones(ctx, "p", "ng1")
	if err != nil || len(zones) == 0 {
		t.Fatalf("%v %#v", err, zones)
	}

	if err := client.DeleteSnapshot(ctx, "p", "ng1", snap.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteBackup(ctx, "p", "ng1", bak.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSnapshot(ctx, "p", "ng1", "missing"); !errors.Is(err, nobusapi.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestNetworkLifecycle(t *testing.T) {
	mock := testapi.New(t)
	srv := mock.Server()
	defer srv.Close()
	client := testClient(t, srv.URL)
	ctx := context.Background()

	bundle, err := client.CreateNetworkBundle(ctx, nobusapi.CreateNetworkRequest{
		ProjectID: "p", AvailabilityZone: "ng1", NetworkName: "n", SubnetCIDR: "10.0.0.0/24", RouterName: "r",
	})
	if err != nil || bundle.NetworkID == "" {
		t.Fatal(err)
	}
	net, err := client.GetNetwork(ctx, "p", "ng1", bundle.NetworkID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateNetwork(ctx, "p", "ng1", bundle.NetworkID, nobusapi.NetworkUpdate{Name: "n2"}); err != nil {
		t.Fatal(err)
	}

	sub, err := client.CreateSubnet(ctx, "p", "ng1", nobusapi.SubnetCreate{
		NetworkID: net.ID, CIDR: "10.1.0.0/24", IPVersion: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSubnet(ctx, "p", "ng1", sub.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateSubnet(ctx, "p", "ng1", sub.ID, nobusapi.SubnetUpdate{Name: "s2"}); err != nil {
		t.Fatal(err)
	}

	port, err := client.CreatePort(ctx, "p", "ng1", nobusapi.PortCreate{NetworkID: net.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetPort(ctx, "p", "ng1", port.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdatePort(ctx, "p", "ng1", port.ID, nobusapi.PortUpdate{Name: "p2"}); err != nil {
		t.Fatal(err)
	}

	rtr, err := client.CreateRouter(ctx, "p", "ng1", nobusapi.RouterWrite{Name: "r2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetRouter(ctx, "p", "ng1", rtr.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateRouter(ctx, "p", "ng1", rtr.ID, nobusapi.RouterWrite{Name: "r3"}); err != nil {
		t.Fatal(err)
	}

	sg, err := client.CreateSecurityGroup(ctx, "p", "ng1", nobusapi.SecurityGroupWrite{Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSecurityGroup(ctx, "p", "ng1", sg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateSecurityGroup(ctx, "p", "ng1", sg.ID, nobusapi.SecurityGroupWrite{Description: "d"}); err != nil {
		t.Fatal(err)
	}

	rule, err := client.CreateSecurityGroupRule(ctx, "p", "ng1", nobusapi.SecurityGroupRuleCreate{
		SecurityGroupID: sg.ID, Direction: "ingress", Ethertype: "IPv4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSecurityGroupRule(ctx, "p", "ng1", rule.ID); err != nil {
		t.Fatal(err)
	}

	fip, err := client.CreateFloatingIP(ctx, "p", "ng1", nobusapi.FloatingIPCreate{FloatingNetworkID: "ext"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetFloatingIP(ctx, "p", "ng1", fip.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateFloatingIP(ctx, "p", "ng1", fip.ID, nobusapi.FloatingIPUpdate{Description: "x"}); err != nil {
		t.Fatal(err)
	}

	_ = client.DeleteSecurityGroupRule(ctx, "p", "ng1", rule.ID)
	_ = client.DeleteSecurityGroup(ctx, "p", "ng1", sg.ID)
	_ = client.DeleteFloatingIP(ctx, "p", "ng1", fip.ID)
	_ = client.DeletePort(ctx, "p", "ng1", port.ID)
	_ = client.DeleteSubnet(ctx, "p", "ng1", sub.ID)
	_ = client.DeleteRouter(ctx, "p", "ng1", rtr.ID)
	_ = client.DeleteNetwork(ctx, "p", "ng1", bundle.NetworkID)
}
