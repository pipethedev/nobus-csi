package nobusapi

import (
	"context"
	"net/http"
)

type NetworkSetup struct {
	NetworkID string `json:"network_id"`
	SubnetID  string `json:"subnet_id"`
	RouterID  string `json:"router_id"`
}

type CreateNetworkRequest struct {
	ProjectID         string   `json:"project_id"`
	AvailabilityZone  string   `json:"availability_zone"`
	NetworkName       string   `json:"network_name"`
	SubnetCIDR        string   `json:"subnet_cidr"`
	RouterName        string   `json:"router_name"`
	ExternalNetworkID string   `json:"external_network_id,omitempty"`
	DNSNameservers    []string `json:"dns_nameservers,omitempty"`
	EnableSNAT        *bool    `json:"enable_snat,omitempty"`
}

type Network struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Status              string   `json:"status"`
	AdminStateUp        *bool    `json:"admin_state_up"`
	Shared              *bool    `json:"shared"`
	MTU                 *int     `json:"mtu"`
	PortSecurityEnabled *bool    `json:"port_security_enabled"`
	Subnets             []string `json:"subnets"`
}

type Subnet struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	NetworkID      string   `json:"network_id"`
	CIDR           string   `json:"cidr"`
	IPVersion      int      `json:"ip_version"`
	GatewayIP      string   `json:"gateway_ip"`
	EnableDHCP     *bool    `json:"enable_dhcp"`
	DNSNameservers []string `json:"dns_nameservers"`
}

type Port struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	NetworkID           string   `json:"network_id"`
	Status              string   `json:"status"`
	AdminStateUp        *bool    `json:"admin_state_up"`
	MACAddress          string   `json:"mac_address"`
	DeviceID            string   `json:"device_id"`
	DeviceOwner         string   `json:"device_owner"`
	SecurityGroups      []string `json:"security_groups"`
	PortSecurityEnabled *bool    `json:"port_security_enabled"`
}

type Router struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	AdminStateUp *bool  `json:"admin_state_up"`
}

type SecurityGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SecurityGroupRule struct {
	ID              string `json:"id"`
	SecurityGroupID string `json:"security_group_id"`
	Direction       string `json:"direction"`
	Ethertype       string `json:"ethertype"`
	Protocol        string `json:"protocol"`
	PortRangeMin    *int   `json:"port_range_min"`
	PortRangeMax    *int   `json:"port_range_max"`
	RemoteIPPrefix  string `json:"remote_ip_prefix"`
	RemoteGroupID   string `json:"remote_group_id"`
	Description     string `json:"description"`
}

type FloatingIP struct {
	ID                string `json:"id"`
	FloatingNetworkID string `json:"floating_network_id"`
	FloatingIPAddress string `json:"floating_ip_address"`
	PortID            string `json:"port_id"`
	FixedIPAddress    string `json:"fixed_ip_address"`
	Status            string `json:"status"`
	Description       string `json:"description"`
}

type NetworkUpdate struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	AdminStateUp *bool  `json:"admin_state_up,omitempty"`
}

type SubnetCreate struct {
	NetworkID      string   `json:"network_id"`
	CIDR           string   `json:"cidr"`
	IPVersion      int      `json:"ip_version"`
	Name           string   `json:"name,omitempty"`
	Description    string   `json:"description,omitempty"`
	GatewayIP      string   `json:"gateway_ip,omitempty"`
	EnableDHCP     *bool    `json:"enable_dhcp,omitempty"`
	DNSNameservers []string `json:"dns_nameservers,omitempty"`
}

type SubnetUpdate struct {
	Name           string   `json:"name,omitempty"`
	Description    string   `json:"description,omitempty"`
	GatewayIP      string   `json:"gateway_ip,omitempty"`
	EnableDHCP     *bool    `json:"enable_dhcp,omitempty"`
	DNSNameservers []string `json:"dns_nameservers,omitempty"`
}

type PortCreate struct {
	NetworkID           string   `json:"network_id"`
	Name                string   `json:"name,omitempty"`
	Description         string   `json:"description,omitempty"`
	AdminStateUp        *bool    `json:"admin_state_up,omitempty"`
	SecurityGroups      []string `json:"security_groups,omitempty"`
	DeviceID            string   `json:"device_id,omitempty"`
	DeviceOwner         string   `json:"device_owner,omitempty"`
	PortSecurityEnabled *bool    `json:"port_security_enabled,omitempty"`
}

type PortUpdate struct {
	Name                string   `json:"name,omitempty"`
	Description         string   `json:"description,omitempty"`
	AdminStateUp        *bool    `json:"admin_state_up,omitempty"`
	SecurityGroups      []string `json:"security_groups,omitempty"`
	PortSecurityEnabled *bool    `json:"port_security_enabled,omitempty"`
}

type RouterWrite struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	AdminStateUp *bool  `json:"admin_state_up,omitempty"`
}

type SecurityGroupWrite struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type SecurityGroupRuleCreate struct {
	SecurityGroupID string `json:"security_group_id"`
	Direction       string `json:"direction"`
	Ethertype       string `json:"ethertype,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	PortRangeMin    *int   `json:"port_range_min,omitempty"`
	PortRangeMax    *int   `json:"port_range_max,omitempty"`
	RemoteIPPrefix  string `json:"remote_ip_prefix,omitempty"`
	RemoteGroupID   string `json:"remote_group_id,omitempty"`
	Description     string `json:"description,omitempty"`
}

type FloatingIPCreate struct {
	FloatingNetworkID string `json:"floating_network_id"`
	PortID            string `json:"port_id,omitempty"`
	FixedIPAddress    string `json:"fixed_ip_address,omitempty"`
	Description       string `json:"description,omitempty"`
}

type FloatingIPUpdate struct {
	PortID         *string `json:"port_id,omitempty"`
	FixedIPAddress string  `json:"fixed_ip_address,omitempty"`
	Description    string  `json:"description,omitempty"`
}

func (c *Client) CreateNetworkBundle(ctx context.Context, req CreateNetworkRequest) (*NetworkSetup, error) {
	var resp struct {
		Status bool         `json:"status"`
		Data   NetworkSetup `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetNetwork(ctx context.Context, projectID, availabilityZone, networkID string) (*Network, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("network_id", networkID)
	var resp struct {
		Status bool    `json:"status"`
		Data   Network `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteNetwork(ctx context.Context, projectID, availabilityZone, networkID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("network_id", networkID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/", q, nil, nil)
}

func (c *Client) UpdateNetwork(ctx context.Context, projectID, availabilityZone, networkID string, network NetworkUpdate) (*Network, error) {
	body := struct {
		ProjectID        string        `json:"project_id"`
		AvailabilityZone string        `json:"availability_zone"`
		NetworkID        string        `json:"network_id"`
		Network          NetworkUpdate `json:"network"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		NetworkID:        networkID,
		Network:          network,
	}
	var resp struct {
		Status bool    `json:"status"`
		Data   Network `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) CreateSubnet(ctx context.Context, projectID, availabilityZone string, subnet SubnetCreate) (*Subnet, error) {
	body := struct {
		ProjectID        string       `json:"project_id"`
		AvailabilityZone string       `json:"availability_zone"`
		Subnet           SubnetCreate `json:"subnet"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		Subnet:           subnet,
	}
	var resp struct {
		Status bool   `json:"status"`
		Data   Subnet `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/subnet", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetSubnet(ctx context.Context, projectID, availabilityZone, subnetID string) (*Subnet, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("subnet_id", subnetID)
	var resp struct {
		Status bool   `json:"status"`
		Data   Subnet `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/subnet", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdateSubnet(ctx context.Context, projectID, availabilityZone, subnetID string, subnet SubnetUpdate) (*Subnet, error) {
	body := struct {
		ProjectID        string       `json:"project_id"`
		AvailabilityZone string       `json:"availability_zone"`
		SubnetID         string       `json:"subnet_id"`
		Subnet           SubnetUpdate `json:"subnet"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		SubnetID:         subnetID,
		Subnet:           subnet,
	}
	var resp struct {
		Status bool   `json:"status"`
		Data   Subnet `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/subnet", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteSubnet(ctx context.Context, projectID, availabilityZone, subnetID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("subnet_id", subnetID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/subnet", q, nil, nil)
}

func (c *Client) CreatePort(ctx context.Context, projectID, availabilityZone string, port PortCreate) (*Port, error) {
	body := struct {
		ProjectID        string     `json:"project_id"`
		AvailabilityZone string     `json:"availability_zone"`
		Port             PortCreate `json:"port"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		Port:             port,
	}
	var resp struct {
		Status bool `json:"status"`
		Data   Port `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/port", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetPort(ctx context.Context, projectID, availabilityZone, portID string) (*Port, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("port_id", portID)
	var resp struct {
		Status bool `json:"status"`
		Data   Port `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/port", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdatePort(ctx context.Context, projectID, availabilityZone, portID string, port PortUpdate) (*Port, error) {
	body := struct {
		ProjectID        string     `json:"project_id"`
		AvailabilityZone string     `json:"availability_zone"`
		PortID           string     `json:"port_id"`
		Port             PortUpdate `json:"port"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		PortID:           portID,
		Port:             port,
	}
	var resp struct {
		Status bool `json:"status"`
		Data   Port `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/port", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeletePort(ctx context.Context, projectID, availabilityZone, portID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("port_id", portID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/port", q, nil, nil)
}

func (c *Client) CreateRouter(ctx context.Context, projectID, availabilityZone string, router RouterWrite) (*Router, error) {
	body := struct {
		ProjectID        string      `json:"project_id"`
		AvailabilityZone string      `json:"availability_zone"`
		Router           RouterWrite `json:"router"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		Router:           router,
	}
	var resp struct {
		Status bool   `json:"status"`
		Data   Router `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/router", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetRouter(ctx context.Context, projectID, availabilityZone, routerID string) (*Router, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("router_id", routerID)
	var resp struct {
		Status bool   `json:"status"`
		Data   Router `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/router", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdateRouter(ctx context.Context, projectID, availabilityZone, routerID string, router RouterWrite) (*Router, error) {
	body := struct {
		ProjectID        string      `json:"project_id"`
		AvailabilityZone string      `json:"availability_zone"`
		RouterID         string      `json:"router_id"`
		Router           RouterWrite `json:"router"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		RouterID:         routerID,
		Router:           router,
	}
	var resp struct {
		Status bool   `json:"status"`
		Data   Router `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/router", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteRouter(ctx context.Context, projectID, availabilityZone, routerID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("router_id", routerID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/router", q, nil, nil)
}

func (c *Client) CreateSecurityGroup(ctx context.Context, projectID, availabilityZone string, securityGroup SecurityGroupWrite) (*SecurityGroup, error) {
	body := struct {
		ProjectID        string             `json:"project_id"`
		AvailabilityZone string             `json:"availability_zone"`
		SecurityGroup    SecurityGroupWrite `json:"security_group"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		SecurityGroup:    securityGroup,
	}
	var resp struct {
		Status bool          `json:"status"`
		Data   SecurityGroup `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/security-group", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetSecurityGroup(ctx context.Context, projectID, availabilityZone, securityGroupID string) (*SecurityGroup, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("security_group_id", securityGroupID)
	var resp struct {
		Status bool          `json:"status"`
		Data   SecurityGroup `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/security-group", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdateSecurityGroup(ctx context.Context, projectID, availabilityZone, securityGroupID string, securityGroup SecurityGroupWrite) (*SecurityGroup, error) {
	body := struct {
		ProjectID        string             `json:"project_id"`
		AvailabilityZone string             `json:"availability_zone"`
		SecurityGroupID  string             `json:"securitygroup_id"`
		SecurityGroup    SecurityGroupWrite `json:"security_group"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		SecurityGroupID:  securityGroupID,
		SecurityGroup:    securityGroup,
	}
	var resp struct {
		Status bool          `json:"status"`
		Data   SecurityGroup `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/security-group", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteSecurityGroup(ctx context.Context, projectID, availabilityZone, securityGroupID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("security_group_id", securityGroupID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/security-group", q, nil, nil)
}

func (c *Client) CreateSecurityGroupRule(ctx context.Context, projectID, availabilityZone string, rule SecurityGroupRuleCreate) (*SecurityGroupRule, error) {
	body := struct {
		ProjectID         string                  `json:"project_id"`
		AvailabilityZone  string                  `json:"availability_zone"`
		SecurityGroupRule SecurityGroupRuleCreate `json:"security_group_rule"`
	}{
		ProjectID:         projectID,
		AvailabilityZone:  availabilityZone,
		SecurityGroupRule: rule,
	}
	var resp struct {
		Status bool              `json:"status"`
		Data   SecurityGroupRule `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/security-group-rule", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetSecurityGroupRule(ctx context.Context, projectID, availabilityZone, ruleID string) (*SecurityGroupRule, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("security_group_rule_id", ruleID)
	var resp struct {
		Status bool              `json:"status"`
		Data   SecurityGroupRule `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/security-group-rule", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteSecurityGroupRule(ctx context.Context, projectID, availabilityZone, ruleID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("security_group_rule_id", ruleID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/security-group-rule", q, nil, nil)
}

func (c *Client) CreateFloatingIP(ctx context.Context, projectID, availabilityZone string, floatingIP FloatingIPCreate) (*FloatingIP, error) {
	body := struct {
		ProjectID        string           `json:"project_id"`
		AvailabilityZone string           `json:"availability_zone"`
		FloatingIP       FloatingIPCreate `json:"floatingip"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		FloatingIP:       floatingIP,
	}
	var resp struct {
		Status bool       `json:"status"`
		Data   FloatingIP `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v3/network/floating-ip", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetFloatingIP(ctx context.Context, projectID, availabilityZone, floatingIPID string) (*FloatingIP, error) {
	q := projectQuery(projectID, availabilityZone)
	q.Set("floating_ip_id", floatingIPID)
	var resp struct {
		Status bool       `json:"status"`
		Data   FloatingIP `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v3/network/floating-ip", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdateFloatingIP(ctx context.Context, projectID, availabilityZone, floatingIPID string, floatingIP FloatingIPUpdate) (*FloatingIP, error) {
	body := struct {
		ProjectID        string           `json:"project_id"`
		AvailabilityZone string           `json:"availability_zone"`
		FloatingIPID     string           `json:"floatingip_id"`
		FloatingIP       FloatingIPUpdate `json:"floatingip"`
	}{
		ProjectID:        projectID,
		AvailabilityZone: availabilityZone,
		FloatingIPID:     floatingIPID,
		FloatingIP:       floatingIP,
	}
	var resp struct {
		Status bool       `json:"status"`
		Data   FloatingIP `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v3/network/floating-ip", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteFloatingIP(ctx context.Context, projectID, availabilityZone, floatingIPID string) error {
	q := projectQuery(projectID, availabilityZone)
	q.Set("floating_ip_id", floatingIPID)
	return c.do(ctx, http.MethodDelete, "/api/v3/network/floating-ip", q, nil, nil)
}
