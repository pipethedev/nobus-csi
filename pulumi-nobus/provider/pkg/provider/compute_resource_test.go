package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestKeyPairCRUD(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()
	var r KeyPair

	args, failures, err := r.Check(ctx, "demo", nil, props(map[string]any{
		"name": "demo-key", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}

	id, state, err := r.Create(ctx, "demo", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" || state.PrivateKey == "" {
		t.Fatalf("%q %#v", id, state)
	}

	id2, inputs, state2, err := r.Read(ctx, id, args, state)
	if err != nil || id2 == "" || inputs.Name != "demo-key" {
		t.Fatalf("%v %q %#v", err, id2, state2)
	}

	diff, err := r.Diff(ctx, id, state2, KeyPairArgs{Name: "other", AvailabilityZone: "ng1", ProjectID: "proj-1"})
	if err != nil || !diff.HasChanges {
		t.Fatalf("%v %#v", err, diff)
	}

	if err := r.Delete(ctx, id, state2); err != nil {
		t.Fatal(err)
	}
	_, _, gone, err := r.Read(ctx, id, args, state2)
	if err != nil {
		t.Fatal(err)
	}
	if gone.KeyPairID != "" && gone.Name != "" {
		// Read returns empty identity when missing
		t.Fatalf("expected deleted, got %#v", gone)
	}
}

func TestKeyPairCheckRequiresAZ(t *testing.T) {
	cfg := &Config{APIKey: "k", ProjectID: "p"} // no AZ default
	ctx := ContextWithConfig(context.Background(), cfg)
	var r KeyPair
	_, failures, err := r.Check(ctx, "x", nil, props(map[string]any{"name": "k"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) == 0 {
		t.Fatal("expected az failure")
	}
}

func TestInstanceCRUDAndUpdate(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()
	var r Instance

	args, failures, err := r.Check(ctx, "vm", nil, props(map[string]any{
		"name": "vm", "image": "img-1", "flavor": "fl-1", "keyName": "k",
		"availabilityZone": "ng1", "securityGroups": []string{"default"}, "tags": []string{"a"},
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}

	previewID, _, err := r.Create(ctx, "vm", args, true)
	if err != nil || previewID == "" {
		t.Fatal(err)
	}

	id, state, err := r.Create(ctx, "vm", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if state.PrivateIPv4 == "" || state.PublicIPv4 == "" {
		t.Fatalf("expected IPs %#v", state)
	}

	id2, _, state2, err := r.Read(ctx, id, args, state)
	if err != nil || id2 == "" {
		t.Fatal(err)
	}

	news := state2.InstanceArgs
	news.Name = "vm-renamed"
	news.Flavor = "fl-2"
	news.SecurityGroups = []string{"web"}
	news.Tags = []string{"b"}
	diff, err := r.Diff(ctx, id, state2, news)
	if err != nil || !diff.HasChanges {
		t.Fatalf("%v %#v", err, diff)
	}
	if diff.DetailedDiff["name"].Kind != p.Update {
		t.Fatalf("name diff %#v", diff.DetailedDiff["name"])
	}
	if diff.DetailedDiff["flavor"].Kind != p.Update {
		t.Fatalf("flavor diff %#v", diff.DetailedDiff["flavor"])
	}

	updated, err := r.Update(ctx, id, state2, news, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "vm-renamed" {
		t.Fatalf("%#v", updated)
	}

	replaceNews := news
	replaceNews.Image = "img-2"
	diff, err = r.Diff(ctx, id, updated, replaceNews)
	if err != nil || diff.DetailedDiff["image"].Kind != p.UpdateReplace {
		t.Fatalf("%v %#v", err, diff)
	}

	if err := r.Delete(ctx, id, updated); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceCheckRequired(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()
	var r Instance
	_, failures, err := r.Check(ctx, "vm", nil, props(map[string]any{
		"availabilityZone": "ng1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) < 3 {
		t.Fatalf("failures=%v", failures)
	}
}

func TestTypesAndCheckHelpers(t *testing.T) {
	if ScopedID("a", "b", "c") != "a/b/c" {
		t.Fatal()
	}
	az, project, id := ParseScopedID("a/b")
	if az != "a" || project != "" || id != "b" {
		t.Fatalf("%q %q %q", az, project, id)
	}
	if requireAZ("") == nil {
		t.Fatal()
	}
	diff := map[string]p.PropertyDiff{}
	replaceDiff("x", true, diff)
	updateDiff("y", true, diff)
	resp := diffResponse(diff)
	if !resp.HasChanges || !resp.DeleteBeforeReplace {
		t.Fatalf("%#v", resp)
	}
	if !mapsEqualTest(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Fatal()
	}
	if intPtrEqual(nil, intPtr(1)) {
		t.Fatal()
	}
	if boolPtrEqual(nil, boolPtr(true)) {
		t.Fatal()
	}
	ids := bareIDs([]string{"az/p/x", "y"})
	if ids[0] != "x" || ids[1] != "y" {
		t.Fatalf("%v", ids)
	}

	cfg := &Config{ProjectID: "p", AvailabilityZone: "ng1"}
	ctx := ContextWithConfig(context.Background(), cfg)
	args, failures, err := checkResourceArgs(ctx, props(map[string]any{"name": "n"}), func(a *KeyPairArgs) string {
		return a.AvailabilityZone
	})
	if err != nil || len(failures) > 0 || args.AvailabilityZone != "ng1" || args.ProjectID != "p" {
		t.Fatalf("%v %v %#v", err, failures, args)
	}
}

func mapsEqualTest(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestConfigResolve(t *testing.T) {
	cfg := &Config{}
	if _, err := cfg.ResolveProject(""); err == nil {
		t.Fatal()
	}
	cfg.ProjectID = "p"
	if _, _, err := cfg.ResolveScope("p", ""); err == nil {
		t.Fatal("az required")
	}
	az, project, resid, err := cfg.ResolveImport("ng1/proj/res", "", "")
	if err != nil || az != "ng1" || project != "proj" || resid != "res" {
		t.Fatalf("%v %s %s %s", err, az, project, resid)
	}

	cfg2 := &Config{APIKey: "k", Endpoint: "http://127.0.0.1:1", ProjectID: "p"}
	if err := cfg2.Configure(context.Background()); err != nil {
		// endpoint may still create client
		_ = err
	}
	cfg3 := &Config{}
	if err := cfg3.Configure(context.Background()); err == nil {
		t.Fatal("expected missing api key")
	}
}

func TestProviderConstructs(t *testing.T) {
	_ = Provider()
	_ = Version
}