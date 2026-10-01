package provider

import (
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestVolumeAndAttachment(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var vol Volume
	args, failures, err := vol.Check(ctx, "v", nil, props(map[string]any{
		"size": 10, "name": "disk", "availabilityZone": "ng1",
		"metadata": map[string]string{"k": "v"},
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	_, badFail, err := vol.Check(ctx, "v", nil, props(map[string]any{
		"size": 0, "availabilityZone": "ng1",
	}))
	if err != nil || len(badFail) == 0 {
		t.Fatalf("expected size failure")
	}

	id, state, err := vol.Create(ctx, "v", args, false)
	if err != nil {
		t.Fatal(err)
	}
	id2, _, state2, err := vol.Read(ctx, id, args, state)
	if err != nil || id2 == "" {
		t.Fatal(err)
	}
	news := state2.VolumeArgs
	news.Name = "disk2"
	news.Description = "d"
	news.Metadata = map[string]string{"k": "2"}
	diff, err := vol.Diff(ctx, id, state2, news)
	if err != nil || !diff.HasChanges {
		t.Fatalf("%v %#v", err, diff)
	}
	updated, err := vol.Update(ctx, id, state2, news, false)
	if err != nil || updated.Name != "disk2" {
		t.Fatalf("%v %#v", err, updated)
	}

	var att VolumeAttachment
	aArgs, failures, err := att.Check(ctx, "a", nil, props(map[string]any{
		"volumeId": id, "instanceId": "srv-1", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	// Create instance for attach target in mock by creating an Instance first
	var inst Instance
	_, istate, err := inst.Create(ctx, "vm", InstanceArgs{
		Name: "vm", Image: "img-1", Flavor: "fl-1", KeyName: "k", AvailabilityZone: "ng1", ProjectID: "proj-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	aArgs.InstanceID = BareResourceID(ScopedID("ng1", "proj-1", "srv-1"))
	// use real created instance id from state via Read path: Instance Create returns srv-1
	_ = istate
	aArgs.InstanceID = "srv-1"
	aArgs.VolumeID = BareResourceID(id)

	aid, astate, err := att.Create(ctx, "a", aArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	if astate.Device == "" {
		t.Fatalf("%#v", astate)
	}
	_, _, astate2, err := att.Read(ctx, aid, aArgs, astate)
	if err != nil || astate2.Device == "" {
		t.Fatalf("%v %#v", err, astate2)
	}
	adiff, err := att.Diff(ctx, aid, astate2, VolumeAttachmentArgs{
		VolumeID: "other", InstanceID: "srv-1", AvailabilityZone: "ng1", ProjectID: "proj-1",
	})
	if err != nil || adiff.DetailedDiff["volumeId"].Kind != p.UpdateReplace {
		t.Fatalf("%v %#v", err, adiff)
	}
	if err := att.Delete(ctx, aid, astate2); err != nil {
		t.Fatal(err)
	}
	if err := vol.Delete(ctx, id, updated); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeSnapshotAndBackup(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var snap VolumeSnapshot
	args, failures, err := snap.Check(ctx, "s", nil, props(map[string]any{
		"volumeId": "vol-1", "name": "snap", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	_, missFail, err := snap.Check(ctx, "s", nil, props(map[string]any{
		"availabilityZone": "ng1",
	}))
	if err != nil || len(missFail) == 0 {
		t.Fatalf("expected volumeId failure")
	}

	id, state, err := snap.Create(ctx, "s", args, false)
	if err != nil || state.Status != "available" {
		t.Fatalf("%v %#v", err, state)
	}
	id2, _, state2, err := snap.Read(ctx, id, args, state)
	if err != nil || id2 == "" || state2.Size == 0 {
		t.Fatalf("%v %#v", err, state2)
	}
	diff, err := snap.Diff(ctx, id, state2, VolumeSnapshotArgs{
		VolumeID: "vol-2", AvailabilityZone: "ng1", ProjectID: "proj-1", Name: "snap",
	})
	if err != nil || diff.DetailedDiff["volumeId"].Kind != p.UpdateReplace {
		t.Fatalf("%v %#v", err, diff)
	}
	if err := snap.Delete(ctx, id, state2); err != nil {
		t.Fatal(err)
	}

	var bak VolumeBackup
	bArgs, failures, err := bak.Check(ctx, "b", nil, props(map[string]any{
		"volumeId": "vol-1", "name": "bak", "availabilityZone": "ng1", "container": "c1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	bid, bstate, err := bak.Create(ctx, "b", bArgs, false)
	if err != nil || bstate.Status != "available" {
		t.Fatalf("%v %#v", err, bstate)
	}
	_, _, bstate2, err := bak.Read(ctx, bid, bArgs, bstate)
	if err != nil || bstate2.Size == 0 {
		t.Fatalf("%v %#v", err, bstate2)
	}
	bdiff, err := bak.Diff(ctx, bid, bstate2, VolumeBackupArgs{
		VolumeID: "vol-1", AvailabilityZone: "ng1", ProjectID: "proj-1", Name: "bak2",
	})
	if err != nil || bdiff.DetailedDiff["name"].Kind != p.UpdateReplace {
		t.Fatalf("%v %#v", err, bdiff)
	}
	if err := bak.Delete(ctx, bid, bstate2); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkResources(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var net Network
	args, failures, err := net.Check(ctx, "n", nil, props(map[string]any{
		"networkName": "n", "subnetCidr": "10.0.0.0/24", "routerName": "r", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	id, state, err := net.Create(ctx, "n", args, false)
	if err != nil || state.SubnetID == "" {
		t.Fatalf("%v %#v", err, state)
	}
	_, _, state2, err := net.Read(ctx, id, args, state)
	if err != nil {
		t.Fatal(err)
	}
	news := state2.NetworkArgs
	news.Name = "n2"
	diff, err := net.Diff(ctx, id, state2, news)
	if err != nil || !diff.HasChanges {
		t.Fatalf("%v %#v", err, diff)
	}
	updated, err := net.Update(ctx, id, state2, news, false)
	if err != nil {
		t.Fatal(err)
	}

	var sub Subnet
	sArgs, failures, err := sub.Check(ctx, "s", nil, props(map[string]any{
		"networkId": BareResourceID(id), "cidr": "10.1.0.0/24", "ipVersion": 4, "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	sid, sstate, err := sub.Create(ctx, "s", sArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, sstate2, err := sub.Read(ctx, sid, sArgs, sstate)
	if err != nil {
		t.Fatal(err)
	}
	sNews := sstate2.SubnetArgs
	sNews.Name = "s2"
	sNews.GatewayIP = "10.1.0.1"
	if _, err := sub.Diff(ctx, sid, sstate2, sNews); err != nil {
		t.Fatal(err)
	}
	if _, err := sub.Update(ctx, sid, sstate2, sNews, false); err != nil {
		t.Fatal(err)
	}

	var port Port
	pArgs, failures, err := port.Check(ctx, "p", nil, props(map[string]any{
		"networkId": BareResourceID(id), "availabilityZone": "ng1", "securityGroups": []string{"sg"},
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	pid, pstate, err := port.Create(ctx, "p", pArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, pstate2, err := port.Read(ctx, pid, pArgs, pstate)
	if err != nil {
		t.Fatal(err)
	}
	pNews := pstate2.PortArgs
	pNews.Name = "p2"
	pNews.SecurityGroups = []string{"sg2"}
	if d, err := port.Diff(ctx, pid, pstate2, pNews); err != nil || !d.HasChanges {
		t.Fatalf("%v %#v", err, d)
	}
	if _, err := port.Update(ctx, pid, pstate2, pNews, false); err != nil {
		t.Fatal(err)
	}

	var rtr Router
	rArgs, failures, err := rtr.Check(ctx, "r", nil, props(map[string]any{
		"name": "r", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	rid, rstate, err := rtr.Create(ctx, "r", rArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, rstate2, err := rtr.Read(ctx, rid, rArgs, rstate)
	if err != nil {
		t.Fatal(err)
	}
	rNews := rstate2.RouterArgs
	rNews.Description = "d"
	if _, err := rtr.Diff(ctx, rid, rstate2, rNews); err != nil {
		t.Fatal(err)
	}
	if _, err := rtr.Update(ctx, rid, rstate2, rNews, false); err != nil {
		t.Fatal(err)
	}

	_ = port.Delete(ctx, pid, pstate2)
	_ = sub.Delete(ctx, sid, sstate2)
	_ = rtr.Delete(ctx, rid, rstate2)
	_ = net.Delete(ctx, id, updated)
}

func TestSecurityResources(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var sg SecurityGroup
	args, failures, err := sg.Check(ctx, "sg", nil, props(map[string]any{
		"name": "web", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	id, state, err := sg.Create(ctx, "sg", args, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, state2, err := sg.Read(ctx, id, args, state)
	if err != nil {
		t.Fatal(err)
	}
	news := state2.SecurityGroupArgs
	news.Description = "desc"
	if d, err := sg.Diff(ctx, id, state2, news); err != nil || !d.HasChanges {
		t.Fatalf("%v %#v", err, d)
	}
	if _, err := sg.Update(ctx, id, state2, news, false); err != nil {
		t.Fatal(err)
	}

	min, max := 22, 22
	var rule SecurityGroupRule
	rArgs, failures, err := rule.Check(ctx, "rule", nil, props(map[string]any{
		"securityGroupId": BareResourceID(id), "direction": "ingress", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	rArgs.PortRangeMin = &min
	rArgs.PortRangeMax = &max
	rArgs.Protocol = "tcp"
	rid, rstate, err := rule.Create(ctx, "rule", rArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, rstate2, err := rule.Read(ctx, rid, rArgs, rstate)
	if err != nil {
		t.Fatal(err)
	}
	rNews := rstate2.SecurityGroupRuleArgs
	rNews.Description = "ssh"
	otherMin := 80
	rNews.PortRangeMin = &otherMin
	if d, err := rule.Diff(ctx, rid, rstate2, rNews); err != nil || !d.HasChanges {
		t.Fatalf("%v %#v", err, d)
	}

	var fip FloatingIp
	fArgs, failures, err := fip.Check(ctx, "fip", nil, props(map[string]any{
		"floatingNetworkId": "ext", "availabilityZone": "ng1",
	}))
	if err != nil || len(failures) > 0 {
		t.Fatalf("%v %v", err, failures)
	}
	fid, fstate, err := fip.Create(ctx, "fip", fArgs, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, fstate2, err := fip.Read(ctx, fid, fArgs, fstate)
	if err != nil {
		t.Fatal(err)
	}
	fNews := fstate2.FloatingIpArgs
	fNews.Description = "x"
	if d, err := fip.Diff(ctx, fid, fstate2, fNews); err != nil || !d.HasChanges {
		t.Fatalf("%v %#v", err, d)
	}
	if _, err := fip.Update(ctx, fid, fstate2, fNews, false); err != nil {
		t.Fatal(err)
	}

	_ = rule.Delete(ctx, rid, rstate2)
	_ = fip.Delete(ctx, fid, fstate2)
	_ = sg.Delete(ctx, id, state2)
}

func TestFunctions(t *testing.T) {
	ctx, _, cleanup := testEnv(t)
	defer cleanup()

	var gf GetFlavor
	res, err := gf.Call(ctx, GetFlavorArgs{NameOrID: "s1.small", AvailabilityZone: "ng1", ProjectID: "proj-1"})
	if err != nil || res.FlavorID != "fl-1" {
		t.Fatalf("%v %#v", err, res)
	}

	var gi GetImage
	img, err := gi.Call(ctx, GetImageArgs{NameOrID: "Ubuntu", AvailabilityZone: "ng1"})
	if err != nil || img.ImageID != "img-1" {
		t.Fatalf("%v %#v", err, img)
	}

	var gp GetProject
	proj, err := gp.Call(ctx, GetProjectArgs{})
	if err != nil || proj.ProjectID == "" {
		t.Fatalf("%v %#v", err, proj)
	}
	proj2, err := gp.Call(ctx, GetProjectArgs{NameOrID: "other"})
	if err != nil || proj2.Name != "other" {
		t.Fatalf("%v %#v", err, proj2)
	}

	var gaz GetAvailabilityZones
	zones, err := gaz.Call(ctx, GetAvailabilityZonesArgs{AvailabilityZone: "ng1", ProjectID: "proj-1"})
	if err != nil || len(zones.Names) == 0 {
		t.Fatalf("%v %#v", err, zones)
	}
}

func TestVolumeAttachmentReadParsesCompositeID(t *testing.T) {
	ctx, mock, cleanup := testEnv(t)
	defer cleanup()
	mock.SeedVolume("vol-9", map[string]any{
		"id":          "vol-9",
		"status":      "in-use",
		"attachments": []map[string]any{{"server_id": "srv-9", "device": "/dev/vdb"}},
	})

	var att VolumeAttachment
	_, args, state, err := att.Read(ctx, ScopedID("ng1", "proj-1", "vol-9:srv-9"), VolumeAttachmentArgs{}, VolumeAttachmentState{})
	if err != nil {
		t.Fatal(err)
	}
	if args.VolumeID != "vol-9" || args.InstanceID != "srv-9" || state.Device != "/dev/vdb" {
		t.Fatalf("%#v %#v", args, state)
	}

	_, _, missing, err := att.Read(ctx, ScopedID("ng1", "proj-1", "vol-9"), VolumeAttachmentArgs{}, VolumeAttachmentState{})
	if err != nil {
		t.Fatal(err)
	}
	if missing.VolumeID != "" {
		t.Fatalf("%#v", missing)
	}
}
