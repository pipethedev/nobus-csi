package provider

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

const Version = "0.1.0"

func Provider() p.Provider {
	return infer.Provider(infer.Options{
		Metadata: schema.Metadata{
			DisplayName:       "Nobus",
			Description:       "A Pulumi provider for Nobus Cloud (servers, SSH keys, volumes, snapshots, backups, networking).",
			Keywords:          []string{"pulumi", "nobus", "cloud", "category/cloud", "kind/native"},
			Homepage:          "https://cloud.nobus.io",
			Repository:        "https://github.com/brimble/pulumi-nobus",
			License:           "Apache-2.0",
			Publisher:         "brimble",
			PluginDownloadURL: "github://api.github.com/brimble/pulumi-nobus",
		},
		Config: infer.Config[*Config](),
		Resources: []infer.InferredResource{
			infer.Resource[*KeyPair, KeyPairArgs, KeyPairState](),
			infer.Resource[*Instance, InstanceArgs, InstanceState](),
			infer.Resource[*Volume, VolumeArgs, VolumeState](),
			infer.Resource[*VolumeAttachment, VolumeAttachmentArgs, VolumeAttachmentState](),
			infer.Resource[*VolumeSnapshot, VolumeSnapshotArgs, VolumeSnapshotState](),
			infer.Resource[*VolumeBackup, VolumeBackupArgs, VolumeBackupState](),
			infer.Resource[*Network, NetworkArgs, NetworkState](),
			infer.Resource[*Subnet, SubnetArgs, SubnetState](),
			infer.Resource[*Port, PortArgs, PortState](),
			infer.Resource[*Router, RouterArgs, RouterState](),
			infer.Resource[*SecurityGroup, SecurityGroupArgs, SecurityGroupState](),
			infer.Resource[*SecurityGroupRule, SecurityGroupRuleArgs, SecurityGroupRuleState](),
			infer.Resource[*FloatingIp, FloatingIpArgs, FloatingIpState](),
		},
		Functions: []infer.InferredFunction{
			infer.Function[*GetFlavor, GetFlavorArgs, GetFlavorResult](),
			infer.Function[*GetImage, GetImageArgs, GetImageResult](),
			infer.Function[*GetProject, GetProjectArgs, GetProjectResult](),
			infer.Function[*GetAvailabilityZones, GetAvailabilityZonesArgs, GetAvailabilityZonesResult](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
	})
}

type Config struct {
	APIKey           string `pulumi:"apiKey" provider:"secret"`
	Endpoint         string `pulumi:"endpoint,optional"`
	ProjectID        string `pulumi:"projectId,optional"`
	AvailabilityZone string `pulumi:"availabilityZone,optional"`

	client *nobusapi.Client
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c, "Configuration for the Nobus provider.")
	a.Describe(&c.APIKey, "Nobus API key (Bearer token).")
	a.Describe(&c.Endpoint, "Nobus API endpoint.")
	a.Describe(&c.ProjectID, "Default project ID when a resource omits projectId.")
	a.Describe(&c.AvailabilityZone, "Optional default availability zone. Prefer setting availabilityZone on each resource (required) — Nobus has multiple AZs.")
	a.SetDefault(&c.Endpoint, nobusapi.DefaultEndpoint, "NOBUS_ENDPOINT")
	a.SetDefault(&c.APIKey, "", "NOBUS_API_KEY")
	a.SetDefault(&c.ProjectID, "", "NOBUS_PROJECT_ID")
	a.SetDefault(&c.AvailabilityZone, "", "NOBUS_AVAILABILITY_ZONE")
}

func (c *Config) Configure(ctx context.Context) error {
	if c.APIKey == "" {
		c.APIKey = os.Getenv("NOBUS_API_KEY")
	}
	if c.Endpoint == "" {
		c.Endpoint = os.Getenv("NOBUS_ENDPOINT")
	}
	if c.ProjectID == "" {
		c.ProjectID = os.Getenv("NOBUS_PROJECT_ID")
	}
	if c.AvailabilityZone == "" {
		c.AvailabilityZone = os.Getenv("NOBUS_AVAILABILITY_ZONE")
	}
	client, err := nobusapi.NewClient(nobusapi.Config{
		Endpoint: c.Endpoint,
		APIKey:   c.APIKey,
	})
	if err != nil {
		return err
	}
	c.client = client
	return nil
}

func getConfig(ctx context.Context) *Config {
	if v, ok := ctx.Value(configContextKey{}).(*Config); ok && v != nil {
		return v
	}
	return infer.GetConfig[*Config](ctx)
}

type configContextKey struct{}

// ContextWithConfig attaches provider config for unit tests (and local harnesses).
func ContextWithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configContextKey{}, cfg)
}

func (c *Config) Client() *nobusapi.Client {
	return c.client
}

// SetClientForTest injects an API client without going through Configure.
func (c *Config) SetClientForTest(client *nobusapi.Client) {
	c.client = client
}

// ResolveProject returns an explicit project ID or the provider default.
func (c *Config) ResolveProject(projectID string) (string, error) {
	if projectID == "" {
		projectID = c.ProjectID
	}
	if projectID == "" {
		return "", fmt.Errorf("projectId is required (set on the resource or provider)")
	}
	return projectID, nil
}

// ResolveScope resolves project + AZ. availabilityZone must be set on the resource
// (or via the optional provider default as a convenience).
func (c *Config) ResolveScope(projectID, availabilityZone string) (string, string, error) {
	projectID, err := c.ResolveProject(projectID)
	if err != nil {
		return "", "", err
	}
	if availabilityZone == "" {
		availabilityZone = c.AvailabilityZone
	}
	if err := requireAZ(availabilityZone); err != nil {
		return "", "", err
	}
	return projectID, availabilityZone, nil
}

// ResolveImport fills AZ/project from a scoped import ID, then provider defaults.
func (c *Config) ResolveImport(id, projectID, availabilityZone string) (string, string, string, error) {
	parsedAZ, parsedProject, resourceID := ParseScopedID(id)
	if availabilityZone == "" {
		availabilityZone = parsedAZ
	}
	if projectID == "" {
		projectID = parsedProject
	}
	projectID, availabilityZone, err := c.ResolveScope(projectID, availabilityZone)
	if err != nil {
		return "", "", "", err
	}
	return availabilityZone, projectID, resourceID, nil
}
