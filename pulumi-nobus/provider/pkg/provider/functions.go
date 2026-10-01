package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

type GetFlavor struct{}

type GetFlavorArgs struct {
	NameOrID         string `pulumi:"nameOrId"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type GetFlavorResult struct {
	FlavorID string `pulumi:"flavorId"`
	Name     string `pulumi:"name"`
	VCPUs    int    `pulumi:"vcpus"`
	RAM      int    `pulumi:"ram"`
	Disk     int    `pulumi:"disk"`
}

func (*GetFlavor) Call(ctx context.Context, input GetFlavorArgs) (GetFlavorResult, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return GetFlavorResult{}, err
	}
	flavor, err := cfg.Client().FindFlavor(ctx, projectID, az, input.NameOrID)
	if err != nil {
		return GetFlavorResult{}, err
	}
	return GetFlavorResult{
		FlavorID: flavor.ID,
		Name:     flavor.Name,
		VCPUs:    flavor.VCPUs,
		RAM:      flavor.RAM,
		Disk:     flavor.Disk,
	}, nil
}

func (*GetFlavor) Annotate(a infer.Annotator) {
	a.Describe(&GetFlavor{}, "Look up a Nobus flavor by name or ID.")
}

type GetImage struct{}

type GetImageArgs struct {
	NameOrID         string `pulumi:"nameOrId"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type GetImageResult struct {
	ImageID string `pulumi:"imageId"`
	Name    string `pulumi:"name"`
}

func (*GetImage) Call(ctx context.Context, input GetImageArgs) (GetImageResult, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return GetImageResult{}, err
	}
	image, err := cfg.Client().FindImage(ctx, projectID, az, input.NameOrID)
	if err != nil {
		return GetImageResult{}, err
	}
	return GetImageResult{ImageID: image.ID, Name: image.Name}, nil
}

func (*GetImage) Annotate(a infer.Annotator) {
	a.Describe(&GetImage{}, "Look up a Nobus image by name or ID.")
}

type GetProject struct{}

type GetProjectArgs struct {
	NameOrID string `pulumi:"nameOrId,optional"`
}

type GetProjectResult struct {
	ProjectID        string `pulumi:"projectId"`
	Name             string `pulumi:"name"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	IsDefault        bool   `pulumi:"isDefault"`
}

func (*GetProject) Call(ctx context.Context, input GetProjectArgs) (GetProjectResult, error) {
	cfg := getConfig(ctx)
	client := cfg.Client()
	if input.NameOrID != "" {
		project, err := client.FindProject(ctx, input.NameOrID)
		if err != nil {
			return GetProjectResult{}, err
		}
		return projectResult(project), nil
	}
	if cfg.ProjectID != "" {
		project, err := client.FindProject(ctx, cfg.ProjectID)
		if err != nil {
			return GetProjectResult{}, err
		}
		return projectResult(project), nil
	}
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return GetProjectResult{}, err
	}
	for _, project := range projects {
		if project.IsDefault {
			return projectResult(&project), nil
		}
	}
	if len(projects) == 0 {
		return GetProjectResult{}, fmt.Errorf("project: %w", nobusapi.ErrNotFound)
	}
	return projectResult(&projects[0]), nil
}

func projectResult(project *nobusapi.Project) GetProjectResult {
	return GetProjectResult{
		ProjectID:        project.ID,
		Name:             project.Name,
		AvailabilityZone: project.AvailabilityZone,
		IsDefault:        project.IsDefault,
	}
}

func (*GetProject) Annotate(a infer.Annotator) {
	a.Describe(&GetProject{}, "Look up a Nobus project by name or ID, or return the default project.")
}

type GetAvailabilityZones struct{}

type GetAvailabilityZonesArgs struct {
	AvailabilityZone string `pulumi:"availabilityZone"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type GetAvailabilityZonesResult struct {
	Names []string `pulumi:"names"`
}

func (*GetAvailabilityZones) Call(ctx context.Context, input GetAvailabilityZonesArgs) (GetAvailabilityZonesResult, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return GetAvailabilityZonesResult{}, err
	}
	zones, err := cfg.Client().ListAvailabilityZones(ctx, projectID, az)
	if err != nil {
		return GetAvailabilityZonesResult{}, err
	}
	names := make([]string, 0, len(zones))
	for _, zone := range zones {
		if zone.Name != "" {
			names = append(names, zone.Name)
		}
	}
	return GetAvailabilityZonesResult{Names: names}, nil
}

func (*GetAvailabilityZones) Annotate(a infer.Annotator) {
	a.Describe(&GetAvailabilityZones{}, "List Nobus availability zones visible to a project.")
}
