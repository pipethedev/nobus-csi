package provider

import (
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestNotFoundReadsAndPreviews(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var kp KeyPair
	_, state, err := kp.Create(ctx, "k", KeyPairArgs{Name: "k", AvailabilityZone: "ng1", ProjectID: "proj-1"}, true)
	if err != nil || state.Name != "k" {
		t.Fatal(err)
	}

	id := ScopedID("ng1", "proj-1", "missing-kp")
	_, _, empty, err := kp.Read(ctx, id, KeyPairArgs{AvailabilityZone: "ng1", ProjectID: "proj-1"}, KeyPairState{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Name != "" {
		t.Fatalf("%#v", empty)
	}

	var inst Instance
	_, _, err = inst.Create(ctx, "vm", InstanceArgs{
		Name: "vm", Image: "img-1", Flavor: "fl-1", KeyName: "k", AvailabilityZone: "ng1", ProjectID: "proj-1",
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	_, _, emptyInst, err := inst.Read(ctx, ScopedID("ng1", "proj-1", "no-such"), InstanceArgs{AvailabilityZone: "ng1", ProjectID: "proj-1"}, InstanceState{})
	if err != nil {
		t.Fatal(err)
	}
	if emptyInst.Status != "" {
		t.Fatalf("%#v", emptyInst)
	}

	var vol Volume
	_, _, err = vol.Create(ctx, "v", VolumeArgs{Size: 10, AvailabilityZone: "ng1", ProjectID: "proj-1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, _, emptyVol, err := vol.Read(ctx, ScopedID("ng1", "proj-1", "no-vol"), VolumeArgs{AvailabilityZone: "ng1", ProjectID: "proj-1"}, VolumeState{})
	if err != nil {
		t.Fatal(err)
	}
	if emptyVol.Status != "" {
		t.Fatalf("%#v", emptyVol)
	}
}

func TestCheckFailures(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	checks := []struct {
		name string
		run  func() []p.CheckFailure
	}{
		{"net", func() []p.CheckFailure {
			var r Network
			_, f, _ := r.Check(ctx, "n", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"subnet", func() []p.CheckFailure {
			var r Subnet
			_, f, _ := r.Check(ctx, "s", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"port", func() []p.CheckFailure {
			var r Port
			_, f, _ := r.Check(ctx, "p", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"router", func() []p.CheckFailure {
			var r Router
			_, f, _ := r.Check(ctx, "r", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"sg", func() []p.CheckFailure {
			var r SecurityGroup
			_, f, _ := r.Check(ctx, "sg", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"rule", func() []p.CheckFailure {
			var r SecurityGroupRule
			_, f, _ := r.Check(ctx, "rule", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"fip", func() []p.CheckFailure {
			var r FloatingIp
			_, f, _ := r.Check(ctx, "f", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
		{"attach", func() []p.CheckFailure {
			var r VolumeAttachment
			_, f, _ := r.Check(ctx, "a", nil, props(map[string]any{"availabilityZone": "ng1"}))
			return f
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.run()) == 0 {
				t.Fatal("expected failures")
			}
		})
	}
}

func TestGetProjectBranches(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var gp GetProject
	byID, err := gp.Call(ctx, GetProjectArgs{NameOrID: "proj-1"})
	if err != nil || byID.ProjectID != "proj-1" {
		t.Fatalf("%v %#v", err, byID)
	}

	cfg := getConfig(ctx)
	cfg.ProjectID = "proj-2"
	fromCfg, err := gp.Call(ctx, GetProjectArgs{})
	if err != nil || fromCfg.ProjectID != "proj-2" {
		t.Fatalf("%v %#v", err, fromCfg)
	}

	cfg.ProjectID = ""
	def, err := gp.Call(ctx, GetProjectArgs{})
	if err != nil || !def.IsDefault {
		t.Fatalf("%v %#v", err, def)
	}
}

func TestUpdatePreviews(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var vol Volume
	id, state, err := vol.Create(ctx, "v", VolumeArgs{Size: 10, Name: "v", AvailabilityZone: "ng1", ProjectID: "proj-1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	news := state.VolumeArgs
	news.Name = "v2"
	preview, err := vol.Update(ctx, id, state, news, true)
	if err != nil || preview.Name != "v2" {
		t.Fatalf("%v %#v", err, preview)
	}

	var net Network
	nid, nstate, err := net.Create(ctx, "n", NetworkArgs{
		NetworkName: "n", SubnetCIDR: "10.0.0.0/24", RouterName: "r", AvailabilityZone: "ng1", ProjectID: "proj-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	nNews := nstate.NetworkArgs
	nNews.Name = "n2"
	if _, err := net.Update(ctx, nid, nstate, nNews, true); err != nil {
		t.Fatal(err)
	}

	var inst Instance
	iid, istate, err := inst.Create(ctx, "vm", InstanceArgs{
		Name: "vm", Image: "img-1", Flavor: "fl-1", KeyName: "k", AvailabilityZone: "ng1", ProjectID: "proj-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	iNews := istate.InstanceArgs
	iNews.Name = "vm2"
	if _, err := inst.Update(ctx, iid, istate, iNews, true); err != nil {
		t.Fatal(err)
	}
}
