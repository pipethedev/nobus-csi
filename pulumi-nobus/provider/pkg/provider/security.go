package provider

import (
	"cmp"
	"context"
	"errors"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

type SecurityGroup struct{}

type SecurityGroupArgs struct {
	Name             string `pulumi:"name"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	Description      string `pulumi:"description,optional"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type SecurityGroupState struct {
	SecurityGroupArgs
}

func (*SecurityGroup) Annotate(a infer.Annotator) {
	a.Describe(&SecurityGroup{}, "A Nobus security group.")
}

func (*SecurityGroup) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (SecurityGroupArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *SecurityGroupArgs) *string { return &a.AvailabilityZone },
		func(a *SecurityGroupArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.Name == "" {
		failures = append(failures, p.CheckFailure{Property: "name", Reason: "name is required"})
	}
	return args, failures, nil
}

func (*SecurityGroup) Create(ctx context.Context, name string, input SecurityGroupArgs, preview bool) (string, SecurityGroupState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", SecurityGroupState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := SecurityGroupState{SecurityGroupArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateSecurityGroup(ctx, projectID, az, nobusapi.SecurityGroupWrite{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return "", SecurityGroupState{}, err
	}
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*SecurityGroup) Read(ctx context.Context, id string, inputs SecurityGroupArgs, state SecurityGroupState) (string, SecurityGroupArgs, SecurityGroupState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetSecurityGroup(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", SecurityGroupArgs{}, SecurityGroupState{}, nil
		}
		return id, inputs, state, err
	}
	state.Name = got.Name
	state.Description = got.Description
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.SecurityGroupArgs, state, nil
}

func (*SecurityGroup) Diff(ctx context.Context, id string, olds SecurityGroupState, news SecurityGroupArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*SecurityGroup) Update(ctx context.Context, id string, olds SecurityGroupState, news SecurityGroupArgs, preview bool) (SecurityGroupState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	state := SecurityGroupState{SecurityGroupArgs: news}
	if preview {
		return state, nil
	}
	body := nobusapi.SecurityGroupWrite{
		Name:        news.Name,
		Description: news.Description,
	}
	if body == (nobusapi.SecurityGroupWrite{}) {
		return state, nil
	}
	_, err = cfg.Client().UpdateSecurityGroup(ctx, projectID, az, resourceID, body)
	return state, err
}

func (*SecurityGroup) Delete(ctx context.Context, id string, state SecurityGroupState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteSecurityGroup(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type SecurityGroupRule struct{}

type SecurityGroupRuleArgs struct {
	SecurityGroupID  string `pulumi:"securityGroupId"`
	Direction        string `pulumi:"direction"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	Ethertype        string `pulumi:"ethertype,optional"`
	Protocol         string `pulumi:"protocol,optional"`
	PortRangeMin     *int   `pulumi:"portRangeMin,optional"`
	PortRangeMax     *int   `pulumi:"portRangeMax,optional"`
	RemoteIPPrefix   string `pulumi:"remoteIpPrefix,optional"`
	RemoteGroupID    string `pulumi:"remoteGroupId,optional"`
	Description      string `pulumi:"description,optional"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type SecurityGroupRuleState struct {
	SecurityGroupRuleArgs
}

func (*SecurityGroupRule) Annotate(a infer.Annotator) {
	a.Describe(&SecurityGroupRule{}, "A Nobus security group rule.")
}

func (*SecurityGroupRule) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (SecurityGroupRuleArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *SecurityGroupRuleArgs) *string { return &a.AvailabilityZone },
		func(a *SecurityGroupRuleArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.SecurityGroupID == "" {
		failures = append(failures, p.CheckFailure{Property: "securityGroupId", Reason: "securityGroupId is required"})
	}
	if args.Direction == "" {
		failures = append(failures, p.CheckFailure{Property: "direction", Reason: "direction is required (ingress or egress)"})
	}
	return args, failures, nil
}

func (*SecurityGroupRule) Create(ctx context.Context, name string, input SecurityGroupRuleArgs, preview bool) (string, SecurityGroupRuleState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", SecurityGroupRuleState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.SecurityGroupID = BareResourceID(input.SecurityGroupID)
	input.RemoteGroupID = BareResourceID(input.RemoteGroupID)
	if input.Ethertype == "" {
		input.Ethertype = "IPv4"
	}
	state := SecurityGroupRuleState{SecurityGroupRuleArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateSecurityGroupRule(ctx, projectID, az, nobusapi.SecurityGroupRuleCreate{
		SecurityGroupID: input.SecurityGroupID,
		Direction:       input.Direction,
		Ethertype:       input.Ethertype,
		Protocol:        input.Protocol,
		PortRangeMin:    input.PortRangeMin,
		PortRangeMax:    input.PortRangeMax,
		RemoteIPPrefix:  input.RemoteIPPrefix,
		RemoteGroupID:   input.RemoteGroupID,
		Description:     input.Description,
	})
	if err != nil {
		return "", SecurityGroupRuleState{}, err
	}
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*SecurityGroupRule) Read(ctx context.Context, id string, inputs SecurityGroupRuleArgs, state SecurityGroupRuleState) (string, SecurityGroupRuleArgs, SecurityGroupRuleState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetSecurityGroupRule(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", SecurityGroupRuleArgs{}, SecurityGroupRuleState{}, nil
		}
		return id, inputs, state, err
	}
	state.SecurityGroupID = got.SecurityGroupID
	state.Direction = got.Direction
	state.Ethertype = got.Ethertype
	state.Protocol = got.Protocol
	state.PortRangeMin = got.PortRangeMin
	state.PortRangeMax = got.PortRangeMax
	state.RemoteIPPrefix = got.RemoteIPPrefix
	state.RemoteGroupID = got.RemoteGroupID
	state.Description = got.Description
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.SecurityGroupRuleArgs, state, nil
}

func (*SecurityGroupRule) Diff(ctx context.Context, id string, olds SecurityGroupRuleState, news SecurityGroupRuleArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("securityGroupId", news.SecurityGroupID != "" && BareResourceID(olds.SecurityGroupID) != BareResourceID(news.SecurityGroupID), diff)
	replaceDiff("direction", news.Direction != "" && olds.Direction != news.Direction, diff)
	replaceDiff("ethertype", news.Ethertype != "" && olds.Ethertype != news.Ethertype, diff)
	replaceDiff("protocol", news.Protocol != "" && olds.Protocol != news.Protocol, diff)
	replaceDiff("portRangeMin", !intPtrEqual(olds.PortRangeMin, news.PortRangeMin), diff)
	replaceDiff("portRangeMax", !intPtrEqual(olds.PortRangeMax, news.PortRangeMax), diff)
	replaceDiff("remoteIpPrefix", news.RemoteIPPrefix != "" && olds.RemoteIPPrefix != news.RemoteIPPrefix, diff)
	replaceDiff("remoteGroupId", news.RemoteGroupID != "" && BareResourceID(olds.RemoteGroupID) != BareResourceID(news.RemoteGroupID), diff)
	replaceDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*SecurityGroupRule) Delete(ctx context.Context, id string, state SecurityGroupRuleState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteSecurityGroupRule(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type FloatingIp struct{}

type FloatingIpArgs struct {
	FloatingNetworkID string `pulumi:"floatingNetworkId"`
	AvailabilityZone  string `pulumi:"availabilityZone"`
	PortID            string `pulumi:"portId,optional"`
	FixedIPAddress    string `pulumi:"fixedIpAddress,optional"`
	Description       string `pulumi:"description,optional"`
	ProjectID         string `pulumi:"projectId,optional"`
}

type FloatingIpState struct {
	FloatingIpArgs
	FloatingIPAddress string `pulumi:"floatingIpAddress"`
	Status            string `pulumi:"status"`
}

func (*FloatingIp) Annotate(a infer.Annotator) {
	a.Describe(&FloatingIp{}, "A Nobus floating IP.")
}

func (*FloatingIp) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (FloatingIpArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *FloatingIpArgs) *string { return &a.AvailabilityZone },
		func(a *FloatingIpArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.FloatingNetworkID == "" {
		failures = append(failures, p.CheckFailure{Property: "floatingNetworkId", Reason: "floatingNetworkId is required"})
	}
	return args, failures, nil
}

func (*FloatingIp) Create(ctx context.Context, name string, input FloatingIpArgs, preview bool) (string, FloatingIpState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", FloatingIpState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.FloatingNetworkID = BareResourceID(input.FloatingNetworkID)
	input.PortID = BareResourceID(input.PortID)
	state := FloatingIpState{FloatingIpArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateFloatingIP(ctx, projectID, az, nobusapi.FloatingIPCreate{
		FloatingNetworkID: input.FloatingNetworkID,
		PortID:            input.PortID,
		FixedIPAddress:    input.FixedIPAddress,
		Description:       input.Description,
	})
	if err != nil {
		return "", FloatingIpState{}, err
	}
	state.FloatingIPAddress = created.FloatingIPAddress
	state.Status = created.Status
	state.PortID = created.PortID
	state.FixedIPAddress = created.FixedIPAddress
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*FloatingIp) Read(ctx context.Context, id string, inputs FloatingIpArgs, state FloatingIpState) (string, FloatingIpArgs, FloatingIpState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetFloatingIP(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", FloatingIpArgs{}, FloatingIpState{}, nil
		}
		return id, inputs, state, err
	}
	state.FloatingNetworkID = got.FloatingNetworkID
	state.PortID = got.PortID
	state.FixedIPAddress = got.FixedIPAddress
	state.Description = got.Description
	state.FloatingIPAddress = got.FloatingIPAddress
	state.Status = got.Status
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.FloatingIpArgs, state, nil
}

func (*FloatingIp) Diff(ctx context.Context, id string, olds FloatingIpState, news FloatingIpArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("floatingNetworkId", news.FloatingNetworkID != "" && BareResourceID(olds.FloatingNetworkID) != BareResourceID(news.FloatingNetworkID), diff)
	updateDiff("portId", BareResourceID(olds.PortID) != BareResourceID(news.PortID), diff)
	updateDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*FloatingIp) Update(ctx context.Context, id string, olds FloatingIpState, news FloatingIpArgs, preview bool) (FloatingIpState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	news.FloatingNetworkID = BareResourceID(cmp.Or(news.FloatingNetworkID, olds.FloatingNetworkID))
	news.PortID = BareResourceID(news.PortID)
	state := FloatingIpState{FloatingIpArgs: news, FloatingIPAddress: olds.FloatingIPAddress, Status: olds.Status}
	if preview {
		return state, nil
	}
	body := nobusapi.FloatingIPUpdate{
		FixedIPAddress: news.FixedIPAddress,
		Description:    news.Description,
	}
	if BareResourceID(news.PortID) != BareResourceID(olds.PortID) {
		portID := news.PortID
		body.PortID = &portID
	}
	if body == (nobusapi.FloatingIPUpdate{}) {
		return state, nil
	}
	got, err := cfg.Client().UpdateFloatingIP(ctx, projectID, az, resourceID, body)
	if err != nil {
		return state, err
	}
	state.FloatingIPAddress = got.FloatingIPAddress
	state.Status = got.Status
	state.PortID = got.PortID
	state.FixedIPAddress = got.FixedIPAddress
	return state, nil
}

func (*FloatingIp) Delete(ctx context.Context, id string, state FloatingIpState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteFloatingIP(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
