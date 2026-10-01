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

type Network struct{}

type NetworkArgs struct {
	NetworkName       string   `pulumi:"networkName"`
	SubnetCIDR        string   `pulumi:"subnetCidr"`
	RouterName        string   `pulumi:"routerName"`
	AvailabilityZone  string   `pulumi:"availabilityZone"`
	ProjectID         string   `pulumi:"projectId,optional"`
	ExternalNetworkID string   `pulumi:"externalNetworkId,optional"`
	DNSNameservers    []string `pulumi:"dnsNameservers,optional"`
	EnableSNAT        *bool    `pulumi:"enableSnat,optional"`
	Name              string   `pulumi:"name,optional"`
	Description       string   `pulumi:"description,optional"`
	AdminStateUp      *bool    `pulumi:"adminStateUp,optional"`
}

type NetworkState struct {
	NetworkArgs
	SubnetID            string   `pulumi:"subnetId"`
	RouterID            string   `pulumi:"routerId"`
	Status              string   `pulumi:"status"`
	MTU                 *int     `pulumi:"mtu"`
	Shared              *bool    `pulumi:"shared"`
	PortSecurityEnabled *bool    `pulumi:"portSecurityEnabled"`
	Subnets             []string `pulumi:"subnets"`
}

func (*Network) Annotate(a infer.Annotator) {
	a.Describe(&Network{}, "A Nobus network bundle (network + subnet + router).")
}

func (*Network) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (NetworkArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *NetworkArgs) *string { return &a.AvailabilityZone },
		func(a *NetworkArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.NetworkName == "" {
		failures = append(failures, p.CheckFailure{Property: "networkName", Reason: "networkName is required"})
	}
	if args.SubnetCIDR == "" {
		failures = append(failures, p.CheckFailure{Property: "subnetCidr", Reason: "subnetCidr is required"})
	}
	if args.RouterName == "" {
		failures = append(failures, p.CheckFailure{Property: "routerName", Reason: "routerName is required"})
	}
	return args, failures, nil
}

func (*Network) Create(ctx context.Context, name string, input NetworkArgs, preview bool) (string, NetworkState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", NetworkState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := NetworkState{NetworkArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateNetworkBundle(ctx, nobusapi.CreateNetworkRequest{
		ProjectID:         projectID,
		AvailabilityZone:  az,
		NetworkName:       input.NetworkName,
		SubnetCIDR:        input.SubnetCIDR,
		RouterName:        input.RouterName,
		ExternalNetworkID: BareResourceID(input.ExternalNetworkID),
		DNSNameservers:    input.DNSNameservers,
		EnableSNAT:        input.EnableSNAT,
	})
	if err != nil {
		return "", NetworkState{}, err
	}
	state.SubnetID = created.SubnetID
	state.RouterID = created.RouterID
	if net, err := cfg.Client().GetNetwork(ctx, projectID, az, created.NetworkID); err == nil {
		state = enrichNetworkState(state, net)
	}
	return ScopedID(az, projectID, created.NetworkID), state, nil
}

func enrichNetworkState(state NetworkState, net *nobusapi.Network) NetworkState {
	state.Status = net.Status
	state.MTU = net.MTU
	state.Shared = net.Shared
	state.PortSecurityEnabled = net.PortSecurityEnabled
	state.Subnets = net.Subnets
	state.AdminStateUp = net.AdminStateUp
	if state.Name == "" {
		state.Name = net.Name
	}
	if state.Description == "" {
		state.Description = net.Description
	}
	return state
}

func (*Network) Read(ctx context.Context, id string, inputs NetworkArgs, state NetworkState) (string, NetworkArgs, NetworkState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetNetwork(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", NetworkArgs{}, NetworkState{}, nil
		}
		return id, inputs, state, err
	}
	args := state.NetworkArgs
	if args.NetworkName == "" {
		args = inputs
	}
	args.ProjectID = projectID
	args.AvailabilityZone = az
	state.NetworkArgs = args
	state = enrichNetworkState(state, got)
	return ScopedID(az, projectID, got.ID), state.NetworkArgs, state, nil
}

func (*Network) Diff(ctx context.Context, id string, olds NetworkState, news NetworkArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("networkName", news.NetworkName != "" && olds.NetworkName != news.NetworkName, diff)
	replaceDiff("subnetCidr", news.SubnetCIDR != "" && olds.SubnetCIDR != news.SubnetCIDR, diff)
	replaceDiff("routerName", news.RouterName != "" && olds.RouterName != news.RouterName, diff)
	replaceDiff("externalNetworkId", news.ExternalNetworkID != "" && BareResourceID(olds.ExternalNetworkID) != BareResourceID(news.ExternalNetworkID), diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*Network) Update(ctx context.Context, id string, olds NetworkState, news NetworkArgs, preview bool) (NetworkState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	state := NetworkState{
		NetworkArgs:         news,
		SubnetID:            olds.SubnetID,
		RouterID:            olds.RouterID,
		Status:              olds.Status,
		MTU:                 olds.MTU,
		Shared:              olds.Shared,
		PortSecurityEnabled: olds.PortSecurityEnabled,
		Subnets:             olds.Subnets,
	}
	if preview {
		return state, nil
	}
	update := nobusapi.NetworkUpdate{
		Name:         news.Name,
		Description:  news.Description,
		AdminStateUp: news.AdminStateUp,
	}
	if update == (nobusapi.NetworkUpdate{}) {
		return state, nil
	}
	got, err := cfg.Client().UpdateNetwork(ctx, projectID, az, resourceID, update)
	if err != nil {
		return state, err
	}
	return enrichNetworkState(state, got), nil
}

func (*Network) Delete(ctx context.Context, id string, state NetworkState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteNetwork(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type Subnet struct{}

type SubnetArgs struct {
	NetworkID        string   `pulumi:"networkId"`
	CIDR             string   `pulumi:"cidr"`
	IPVersion        int      `pulumi:"ipVersion"`
	AvailabilityZone string   `pulumi:"availabilityZone"`
	Name             string   `pulumi:"name,optional"`
	Description      string   `pulumi:"description,optional"`
	GatewayIP        string   `pulumi:"gatewayIp,optional"`
	EnableDHCP       *bool    `pulumi:"enableDhcp,optional"`
	DNSNameservers   []string `pulumi:"dnsNameservers,optional"`
	ProjectID        string   `pulumi:"projectId,optional"`
}

type SubnetState struct {
	SubnetArgs
}

func (*Subnet) Annotate(a infer.Annotator) {
	a.Describe(&Subnet{}, "A Nobus subnet.")
}

func (*Subnet) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (SubnetArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *SubnetArgs) *string { return &a.AvailabilityZone },
		func(a *SubnetArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.NetworkID == "" {
		failures = append(failures, p.CheckFailure{Property: "networkId", Reason: "networkId is required"})
	}
	if args.CIDR == "" {
		failures = append(failures, p.CheckFailure{Property: "cidr", Reason: "cidr is required"})
	}
	return args, failures, nil
}

func (*Subnet) Create(ctx context.Context, name string, input SubnetArgs, preview bool) (string, SubnetState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", SubnetState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.NetworkID = BareResourceID(input.NetworkID)
	if input.IPVersion == 0 {
		input.IPVersion = 4
	}
	state := SubnetState{SubnetArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateSubnet(ctx, projectID, az, nobusapi.SubnetCreate{
		NetworkID:      input.NetworkID,
		CIDR:           input.CIDR,
		IPVersion:      input.IPVersion,
		Name:           input.Name,
		Description:    input.Description,
		GatewayIP:      input.GatewayIP,
		EnableDHCP:     input.EnableDHCP,
		DNSNameservers: input.DNSNameservers,
	})
	if err != nil {
		return "", SubnetState{}, err
	}
	if state.Name == "" {
		state.Name = created.Name
	}
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*Subnet) Read(ctx context.Context, id string, inputs SubnetArgs, state SubnetState) (string, SubnetArgs, SubnetState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetSubnet(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", SubnetArgs{}, SubnetState{}, nil
		}
		return id, inputs, state, err
	}
	state.NetworkID = got.NetworkID
	state.CIDR = got.CIDR
	state.IPVersion = got.IPVersion
	state.Name = got.Name
	state.Description = got.Description
	state.GatewayIP = got.GatewayIP
	state.EnableDHCP = got.EnableDHCP
	state.DNSNameservers = got.DNSNameservers
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.SubnetArgs, state, nil
}

func (*Subnet) Diff(ctx context.Context, id string, olds SubnetState, news SubnetArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("networkId", news.NetworkID != "" && BareResourceID(olds.NetworkID) != BareResourceID(news.NetworkID), diff)
	replaceDiff("cidr", news.CIDR != "" && olds.CIDR != news.CIDR, diff)
	replaceDiff("ipVersion", news.IPVersion != 0 && olds.IPVersion != news.IPVersion, diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("gatewayIp", olds.GatewayIP != news.GatewayIP, diff)
	return diffResponse(diff), nil
}

func (*Subnet) Update(ctx context.Context, id string, olds SubnetState, news SubnetArgs, preview bool) (SubnetState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	news.NetworkID = BareResourceID(cmp.Or(news.NetworkID, olds.NetworkID))
	state := SubnetState{SubnetArgs: news}
	if preview {
		return state, nil
	}
	body := nobusapi.SubnetUpdate{
		Name:           news.Name,
		Description:    news.Description,
		GatewayIP:      news.GatewayIP,
		EnableDHCP:     news.EnableDHCP,
		DNSNameservers: news.DNSNameservers,
	}
	if body.Name == "" && body.Description == "" && body.GatewayIP == "" && body.EnableDHCP == nil && len(body.DNSNameservers) == 0 {
		return state, nil
	}
	_, err = cfg.Client().UpdateSubnet(ctx, projectID, az, resourceID, body)
	return state, err
}

func (*Subnet) Delete(ctx context.Context, id string, state SubnetState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteSubnet(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type Port struct{}

type PortArgs struct {
	NetworkID           string   `pulumi:"networkId"`
	AvailabilityZone    string   `pulumi:"availabilityZone"`
	Name                string   `pulumi:"name,optional"`
	Description         string   `pulumi:"description,optional"`
	AdminStateUp        *bool    `pulumi:"adminStateUp,optional"`
	SecurityGroups      []string `pulumi:"securityGroups,optional"`
	DeviceID            string   `pulumi:"deviceId,optional"`
	DeviceOwner         string   `pulumi:"deviceOwner,optional"`
	PortSecurityEnabled *bool    `pulumi:"portSecurityEnabled,optional"`
	ProjectID           string   `pulumi:"projectId,optional"`
}

type PortState struct {
	PortArgs
	Status     string `pulumi:"status"`
	MACAddress string `pulumi:"macAddress"`
}

func (*Port) Annotate(a infer.Annotator) {
	a.Describe(&Port{}, "A Nobus network port.")
}

func (*Port) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (PortArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *PortArgs) *string { return &a.AvailabilityZone },
		func(a *PortArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.NetworkID == "" {
		failures = append(failures, p.CheckFailure{Property: "networkId", Reason: "networkId is required"})
	}
	return args, failures, nil
}

func (*Port) Create(ctx context.Context, name string, input PortArgs, preview bool) (string, PortState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", PortState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.NetworkID = BareResourceID(input.NetworkID)
	input.DeviceID = BareResourceID(input.DeviceID)
	input.SecurityGroups = bareIDs(input.SecurityGroups)
	state := PortState{PortArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreatePort(ctx, projectID, az, nobusapi.PortCreate{
		NetworkID:           input.NetworkID,
		Name:                input.Name,
		Description:         input.Description,
		AdminStateUp:        input.AdminStateUp,
		SecurityGroups:      input.SecurityGroups,
		DeviceID:            input.DeviceID,
		DeviceOwner:         input.DeviceOwner,
		PortSecurityEnabled: input.PortSecurityEnabled,
	})
	if err != nil {
		return "", PortState{}, err
	}
	state.Status = created.Status
	state.MACAddress = created.MACAddress
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*Port) Read(ctx context.Context, id string, inputs PortArgs, state PortState) (string, PortArgs, PortState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetPort(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", PortArgs{}, PortState{}, nil
		}
		return id, inputs, state, err
	}
	state.NetworkID = got.NetworkID
	state.Name = got.Name
	state.Description = got.Description
	state.AdminStateUp = got.AdminStateUp
	state.SecurityGroups = got.SecurityGroups
	state.DeviceID = got.DeviceID
	state.DeviceOwner = got.DeviceOwner
	state.PortSecurityEnabled = got.PortSecurityEnabled
	state.Status = got.Status
	state.MACAddress = got.MACAddress
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.PortArgs, state, nil
}

func (*Port) Diff(ctx context.Context, id string, olds PortState, news PortArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("networkId", news.NetworkID != "" && BareResourceID(olds.NetworkID) != BareResourceID(news.NetworkID), diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("deviceId", BareResourceID(olds.DeviceID) != BareResourceID(news.DeviceID), diff)
	updateDiff("securityGroups", !stringSlicesEqual(olds.SecurityGroups, news.SecurityGroups), diff)
	updateDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*Port) Update(ctx context.Context, id string, olds PortState, news PortArgs, preview bool) (PortState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	news.NetworkID = BareResourceID(cmp.Or(news.NetworkID, olds.NetworkID))
	news.SecurityGroups = bareIDs(news.SecurityGroups)
	state := PortState{PortArgs: news, Status: olds.Status, MACAddress: olds.MACAddress}
	if preview {
		return state, nil
	}
	body := nobusapi.PortUpdate{
		Name:                news.Name,
		Description:         news.Description,
		AdminStateUp:        news.AdminStateUp,
		SecurityGroups:      news.SecurityGroups,
		PortSecurityEnabled: news.PortSecurityEnabled,
	}
	if body.Name == "" && body.Description == "" && body.AdminStateUp == nil && len(body.SecurityGroups) == 0 && body.PortSecurityEnabled == nil {
		return state, nil
	}
	got, err := cfg.Client().UpdatePort(ctx, projectID, az, resourceID, body)
	if err != nil {
		return state, err
	}
	state.Status = got.Status
	state.MACAddress = got.MACAddress
	return state, nil
}

func (*Port) Delete(ctx context.Context, id string, state PortState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeletePort(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type Router struct{}

type RouterArgs struct {
	Name             string `pulumi:"name"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	Description      string `pulumi:"description,optional"`
	AdminStateUp     *bool  `pulumi:"adminStateUp,optional"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type RouterState struct {
	RouterArgs
	Status string `pulumi:"status"`
}

func (*Router) Annotate(a infer.Annotator) {
	a.Describe(&Router{}, "A Nobus router.")
}

func (*Router) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (RouterArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs,
		func(a *RouterArgs) *string { return &a.AvailabilityZone },
		func(a *RouterArgs) *string { return &a.ProjectID },
	)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.Name == "" {
		failures = append(failures, p.CheckFailure{Property: "name", Reason: "name is required"})
	}
	return args, failures, nil
}

func (*Router) Create(ctx context.Context, name string, input RouterArgs, preview bool) (string, RouterState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", RouterState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := RouterState{RouterArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateRouter(ctx, projectID, az, nobusapi.RouterWrite{
		Name:         input.Name,
		Description:  input.Description,
		AdminStateUp: input.AdminStateUp,
	})
	if err != nil {
		return "", RouterState{}, err
	}
	state.Status = created.Status
	return ScopedID(az, projectID, created.ID), state, nil
}

func (*Router) Read(ctx context.Context, id string, inputs RouterArgs, state RouterState) (string, RouterArgs, RouterState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetRouter(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", RouterArgs{}, RouterState{}, nil
		}
		return id, inputs, state, err
	}
	state.Name = got.Name
	state.Description = got.Description
	state.AdminStateUp = got.AdminStateUp
	state.Status = got.Status
	state.ProjectID = projectID
	state.AvailabilityZone = az
	return ScopedID(az, projectID, got.ID), state.RouterArgs, state, nil
}

func (*Router) Diff(ctx context.Context, id string, olds RouterState, news RouterArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("description", olds.Description != news.Description, diff)
	return diffResponse(diff), nil
}

func (*Router) Update(ctx context.Context, id string, olds RouterState, news RouterArgs, preview bool) (RouterState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	state := RouterState{RouterArgs: news, Status: olds.Status}
	if preview {
		return state, nil
	}
	body := nobusapi.RouterWrite{
		Name:         news.Name,
		Description:  news.Description,
		AdminStateUp: news.AdminStateUp,
	}
	if body == (nobusapi.RouterWrite{}) {
		return state, nil
	}
	got, err := cfg.Client().UpdateRouter(ctx, projectID, az, resourceID, body)
	if err != nil {
		return state, err
	}
	state.Status = got.Status
	return state, nil
}

func (*Router) Delete(ctx context.Context, id string, state RouterState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteRouter(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
