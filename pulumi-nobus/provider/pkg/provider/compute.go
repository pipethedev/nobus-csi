package provider

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

type KeyPair struct{}

type KeyPairArgs struct {
	Name             string `pulumi:"name"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type KeyPairState struct {
	KeyPairArgs
	KeyPairID   string `pulumi:"keyPairId"`
	PublicKey   string `pulumi:"publicKey"`
	PrivateKey  string `pulumi:"privateKey" provider:"secret"`
	Fingerprint string `pulumi:"fingerprint"`
	Type        string `pulumi:"type"`
}

func (*KeyPair) Annotate(a infer.Annotator) {
	a.Describe(&KeyPair{}, "An SSH key pair for Nobus instances.")
}

func (*KeyPair) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (KeyPairArgs, []p.CheckFailure, error) {
	return checkResourceArgs(ctx, newInputs, func(a *KeyPairArgs) string { return a.AvailabilityZone })
}

func (*KeyPair) Create(ctx context.Context, name string, input KeyPairArgs, preview bool) (string, KeyPairState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", KeyPairState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := KeyPairState{KeyPairArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateKeyPair(ctx, nobusapi.CreateKeyPairRequest{
		Name:             input.Name,
		ProjectID:        projectID,
		AvailabilityZone: az,
	})
	if err != nil {
		return "", KeyPairState{}, err
	}
	id := created.ID
	if id == "" {
		id = created.Name
	}
	state.KeyPairID = id
	state.PublicKey = created.PublicKey
	state.PrivateKey = created.PrivateKey
	state.Fingerprint = created.Fingerprint
	state.Type = created.Type
	if state.Name == "" {
		state.Name = created.Name
	}
	return ScopedID(az, projectID, id), state, nil
}

func (*KeyPair) Read(ctx context.Context, id string, inputs KeyPairArgs, state KeyPairState) (string, KeyPairArgs, KeyPairState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetKeyPair(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", KeyPairArgs{}, KeyPairState{}, nil
		}
		return id, inputs, state, err
	}
	state.KeyPairID = got.ID
	if state.KeyPairID == "" {
		state.KeyPairID = got.Name
	}
	state.Name = got.Name
	state.PublicKey = got.PublicKey
	if got.PrivateKey != "" {
		state.PrivateKey = got.PrivateKey
	}
	state.Fingerprint = got.Fingerprint
	state.Type = got.Type
	state.ProjectID = projectID
	state.AvailabilityZone = az
	inputs = state.KeyPairArgs
	return ScopedID(az, projectID, state.KeyPairID), inputs, state, nil
}

func (*KeyPair) Diff(ctx context.Context, id string, olds KeyPairState, news KeyPairArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", olds.AvailabilityZone != news.AvailabilityZone && news.AvailabilityZone != "", diff)
	replaceDiff("name", olds.Name != news.Name && news.Name != "", diff)
	replaceDiff("projectId", olds.ProjectID != news.ProjectID && news.ProjectID != "", diff)
	return diffResponse(diff), nil
}

func (*KeyPair) Delete(ctx context.Context, id string, state KeyPairState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteKeyPair(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type Instance struct{}

type InstanceArgs struct {
	Name             string   `pulumi:"name"`
	Image            string   `pulumi:"image"`
	Flavor           string   `pulumi:"flavor"`
	KeyName          string   `pulumi:"keyName"`
	AvailabilityZone string   `pulumi:"availabilityZone"`
	ProjectID        string   `pulumi:"projectId,optional"`
	NetworkID        string   `pulumi:"networkId,optional"`
	VolumeSize       *int     `pulumi:"volumeSize,optional"`
	SecurityGroups   []string `pulumi:"securityGroups,optional"`
	Tags             []string `pulumi:"tags,optional"`
	UserData         string   `pulumi:"userData,optional"`
	AdminPass        string   `pulumi:"adminPass,optional" provider:"secret"`
	DeleteVolume     *bool    `pulumi:"deleteVolume,optional"`
}

type InstanceState struct {
	InstanceArgs
	Status        string   `pulumi:"status"`
	Created       string   `pulumi:"created"`
	Updated       string   `pulumi:"updated"`
	PrivateIPv4   string   `pulumi:"privateIpv4"`
	PublicIPv4    string   `pulumi:"publicIpv4"`
	IPv4Addresses []string `pulumi:"ipv4Addresses"`
	IPv6Addresses []string `pulumi:"ipv6Addresses"`
	Networks      []string `pulumi:"networks"`
}

func (*Instance) Annotate(a infer.Annotator) {
	a.Describe(&Instance{}, "A Nobus compute instance (server).")
}

func (*Instance) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (InstanceArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs, func(a *InstanceArgs) string { return a.AvailabilityZone })
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if strings.TrimSpace(args.Name) == "" {
		failures = append(failures, p.CheckFailure{Property: "name", Reason: "name is required"})
	}
	if strings.TrimSpace(args.Image) == "" {
		failures = append(failures, p.CheckFailure{Property: "image", Reason: "image is required"})
	}
	if strings.TrimSpace(args.Flavor) == "" {
		failures = append(failures, p.CheckFailure{Property: "flavor", Reason: "flavor is required"})
	}
	if strings.TrimSpace(args.KeyName) == "" {
		failures = append(failures, p.CheckFailure{Property: "keyName", Reason: "keyName is required"})
	}
	return args, failures, nil
}

func (*Instance) Create(ctx context.Context, name string, input InstanceArgs, preview bool) (string, InstanceState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", InstanceState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := InstanceState{InstanceArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateInstance(ctx, nobusapi.CreateInstanceRequest{
		ProjectID:        projectID,
		AvailabilityZone: az,
		Name:             input.Name,
		Image:            BareResourceID(input.Image),
		Flavor:           BareResourceID(input.Flavor),
		KeyName:          input.KeyName,
		VolumeSize:       input.VolumeSize,
		NetworkID:        BareResourceID(input.NetworkID),
		SecurityGroups:   bareIDs(input.SecurityGroups),
		AdminPass:        input.AdminPass,
		UserData:         input.UserData,
	})
	if err != nil {
		return "", InstanceState{}, err
	}
	active, err := cfg.Client().WaitInstanceStatus(ctx, projectID, az, created.ID, "ACTIVE")
	if err != nil {
		return ScopedID(az, projectID, created.ID), stateFromInstance(input, created), fmt.Errorf("wait for instance ACTIVE: %w", err)
	}
	if len(input.Tags) > 0 {
		if err := cfg.Client().SetTags(ctx, nobusapi.SetTagsRequest{
			ProjectID:        projectID,
			AvailabilityZone: az,
			ServerID:         active.ID,
			Tags:             input.Tags,
		}); err != nil {
			return ScopedID(az, projectID, active.ID), stateFromInstance(input, active), fmt.Errorf("set tags: %w", err)
		}
	}
	return ScopedID(az, projectID, active.ID), stateFromInstance(input, active), nil
}

func stateFromInstance(input InstanceArgs, inst *nobusapi.Instance) InstanceState {
	parsed := nobusapi.ParseAddresses(inst.Addresses)
	return InstanceState{
		InstanceArgs:  input,
		Status:        inst.Status,
		Created:       inst.Created,
		Updated:       inst.Updated,
		PrivateIPv4:   parsed.PrivateIPv4,
		PublicIPv4:    parsed.PublicIPv4,
		IPv4Addresses: parsed.IPv4,
		IPv6Addresses: parsed.IPv6,
		Networks:      parsed.Networks,
	}
}

func (*Instance) Read(ctx context.Context, id string, inputs InstanceArgs, state InstanceState) (string, InstanceArgs, InstanceState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetInstance(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", InstanceArgs{}, InstanceState{}, nil
		}
		return id, inputs, state, err
	}
	args := state.InstanceArgs
	if args.Name == "" {
		args = inputs
	}
	if args.Name == "" {
		args.Name = got.Name
	}
	if args.KeyName == "" {
		args.KeyName = got.KeyName
	}
	args.ProjectID = projectID
	args.AvailabilityZone = az
	state = stateFromInstance(args, got)
	return ScopedID(az, projectID, got.ID), state.InstanceArgs, state, nil
}

func (*Instance) Diff(ctx context.Context, id string, olds InstanceState, news InstanceArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("image", news.Image != "" && BareResourceID(olds.Image) != BareResourceID(news.Image), diff)
	replaceDiff("keyName", news.KeyName != "" && olds.KeyName != news.KeyName, diff)
	replaceDiff("networkId", news.NetworkID != "" && BareResourceID(olds.NetworkID) != BareResourceID(news.NetworkID), diff)
	replaceDiff("volumeSize", news.VolumeSize != nil && !intPtrEqual(olds.VolumeSize, news.VolumeSize), diff)
	replaceDiff("userData", olds.UserData != news.UserData, diff)
	replaceDiff("adminPass", news.AdminPass != "" && olds.AdminPass != news.AdminPass, diff)

	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("flavor", news.Flavor != "" && BareResourceID(olds.Flavor) != BareResourceID(news.Flavor), diff)
	updateDiff("securityGroups", !stringSlicesEqual(olds.SecurityGroups, news.SecurityGroups), diff)
	updateDiff("tags", !stringSlicesEqual(olds.Tags, news.Tags), diff)
	updateDiff("deleteVolume", !boolPtrEqual(olds.DeleteVolume, news.DeleteVolume), diff)
	return diffResponse(diff), nil
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (*Instance) Update(ctx context.Context, id string, olds InstanceState, news InstanceArgs, preview bool) (InstanceState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	state := stateFromInstance(news, &nobusapi.Instance{
		Status:  olds.Status,
		Created: olds.Created,
		Updated: olds.Updated,
	})
	state.PrivateIPv4 = olds.PrivateIPv4
	state.PublicIPv4 = olds.PublicIPv4
	state.IPv4Addresses = olds.IPv4Addresses
	state.IPv6Addresses = olds.IPv6Addresses
	state.Networks = olds.Networks
	if preview {
		return state, nil
	}

	client := cfg.Client()
	if news.Name != "" && news.Name != olds.Name {
		if err := client.UpdateInstance(ctx, nobusapi.UpdateInstanceRequest{
			ProjectID:        projectID,
			AvailabilityZone: az,
			ServerID:         resourceID,
			Name:             news.Name,
		}); err != nil {
			return state, fmt.Errorf("update instance name: %w", err)
		}
	}

	if news.Flavor != "" && BareResourceID(olds.Flavor) != BareResourceID(news.Flavor) {
		if err := client.ResizeInstance(ctx, nobusapi.ResizeInstanceRequest{
			ProjectID:        projectID,
			AvailabilityZone: az,
			ServerID:         resourceID,
			FlavorID:         BareResourceID(news.Flavor),
		}); err != nil {
			return state, fmt.Errorf("resize instance: %w", err)
		}
		if _, err := client.WaitInstanceStatus(ctx, projectID, az, resourceID, "VERIFY_RESIZE", "ACTIVE"); err != nil {
			return state, fmt.Errorf("wait for resize: %w", err)
		}
		got, err := client.GetInstance(ctx, projectID, az, resourceID)
		if err != nil {
			return state, err
		}
		if strings.EqualFold(got.Status, "VERIFY_RESIZE") {
			if err := client.ConfirmResize(ctx, nobusapi.BaseServerRequest{
				ProjectID:        projectID,
				AvailabilityZone: az,
				ServerID:         resourceID,
			}); err != nil {
				return state, fmt.Errorf("confirm resize: %w", err)
			}
			if _, err := client.WaitInstanceStatus(ctx, projectID, az, resourceID, "ACTIVE"); err != nil {
				return state, fmt.Errorf("wait after confirm resize: %w", err)
			}
		}
	}

	if !stringSlicesEqual(olds.SecurityGroups, news.SecurityGroups) {
		add, remove := sliceDiff(olds.SecurityGroups, news.SecurityGroups)
		for _, sg := range remove {
			if err := client.RemoveSecurityGroup(ctx, nobusapi.SecurityGroupActionRequest{
				ProjectID:         projectID,
				AvailabilityZone:  az,
				ServerID:          resourceID,
				SecurityGroupName: sg,
			}); err != nil {
				return state, fmt.Errorf("remove security group %s: %w", sg, err)
			}
		}
		for _, sg := range add {
			if err := client.AttachSecurityGroup(ctx, nobusapi.SecurityGroupActionRequest{
				ProjectID:         projectID,
				AvailabilityZone:  az,
				ServerID:          resourceID,
				SecurityGroupName: sg,
			}); err != nil {
				return state, fmt.Errorf("attach security group %s: %w", sg, err)
			}
		}
	}

	if !stringSlicesEqual(olds.Tags, news.Tags) {
		if err := client.SetTags(ctx, nobusapi.SetTagsRequest{
			ProjectID:        projectID,
			AvailabilityZone: az,
			ServerID:         resourceID,
			Tags:             news.Tags,
		}); err != nil {
			return state, fmt.Errorf("set tags: %w", err)
		}
	}

	got, err := client.GetInstance(ctx, projectID, az, resourceID)
	if err != nil {
		return state, err
	}
	return stateFromInstance(news, got), nil
}

func (*Instance) Delete(ctx context.Context, id string, state InstanceState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	deleteVolume := state.DeleteVolume != nil && *state.DeleteVolume
	err = cfg.Client().DeleteInstance(ctx, projectID, az, resourceID, deleteVolume)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
