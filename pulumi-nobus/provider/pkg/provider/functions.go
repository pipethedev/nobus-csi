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
		return GetProjectResult{
			ProjectID:        project.ID,
			Name:             project.Name,
			AvailabilityZone: project.AvailabilityZone,
			IsDefault:        project.IsDefault,
		}, nil
	}
	if cfg.ProjectID != "" {
		project, err := client.FindProject(ctx, cfg.ProjectID)
		if err != nil {
			return GetProjectResult{}, err
		}
		return GetProjectResult{
			ProjectID:        project.ID,
			Name:             project.Name,
			AvailabilityZone: project.AvailabilityZone,
			IsDefault:        project.IsDefault,
		}, nil
	}
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return GetProjectResult{}, err
	}
	for _, project := range projects {
		if project.IsDefault {
			return GetProjectResult{
				ProjectID:        project.ID,
				Name:             project.Name,
				AvailabilityZone: project.AvailabilityZone,
				IsDefault:        project.IsDefault,
			}, nil
		}
	}
	if len(projects) == 0 {
		return GetProjectResult{}, fmt.Errorf("project: %w", nobusapi.ErrNotFound)
	}
	project := projects[0]
	return GetProjectResult{
		ProjectID:        project.ID,
		Name:             project.Name,
		AvailabilityZone: project.AvailabilityZone,
		IsDefault:        project.IsDefault,
	}, nil
}

func (*GetProject) Annotate(a infer.Annotator) {
	a.Describe(&GetProject{}, "Look up a Nobus project by name or ID, or return the default project.")
}
