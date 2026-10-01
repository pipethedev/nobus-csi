package provider

import (
	"cmp"
	"context"
	"errors"
	"maps"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

type VolumeBackup struct{}

type VolumeBackupArgs struct {
	VolumeID         string            `pulumi:"volumeId"`
	AvailabilityZone string            `pulumi:"availabilityZone"`
	Name             string            `pulumi:"name,optional"`
	Description      string            `pulumi:"description,optional"`
	Container        string            `pulumi:"container,optional"`
	Incremental      bool              `pulumi:"incremental,optional"`
	Force            bool              `pulumi:"force,optional"`
	SnapshotID       string            `pulumi:"snapshotId,optional"`
	Metadata         map[string]string `pulumi:"metadata,optional"`
	ProjectID        string            `pulumi:"projectId,optional"`
}

type VolumeBackupState struct {
	VolumeBackupArgs
	Status    string `pulumi:"status"`
	Size      int    `pulumi:"size"`
	CreatedAt string `pulumi:"createdAt"`
}

func (*VolumeBackup) Annotate(a infer.Annotator) {
	a.Describe(&VolumeBackup{}, "A Nobus volume backup.")
}

func (*VolumeBackup) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (VolumeBackupArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs, func(a *VolumeBackupArgs) string { return a.AvailabilityZone })
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.VolumeID == "" {
		failures = append(failures, p.CheckFailure{Property: "volumeId", Reason: "volumeId is required"})
	}
	return args, failures, nil
}

func (*VolumeBackup) Create(ctx context.Context, name string, input VolumeBackupArgs, preview bool) (string, VolumeBackupState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", VolumeBackupState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.VolumeID = BareResourceID(input.VolumeID)
	input.SnapshotID = BareResourceID(input.SnapshotID)
	state := VolumeBackupState{VolumeBackupArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateBackup(ctx, nobusapi.CreateBackupRequest{
		VolumeID:         input.VolumeID,
		Name:             input.Name,
		Description:      input.Description,
		Container:        input.Container,
		Incremental:      input.Incremental,
		Force:            input.Force,
		SnapshotID:       input.SnapshotID,
		Metadata:         input.Metadata,
		ProjectID:        projectID,
		AvailabilityZone: az,
	})
	if err != nil {
		return "", VolumeBackupState{}, err
	}
	ready, err := cfg.Client().WaitBackupStatus(ctx, projectID, az, created.ID, "available")
	if err != nil {
		return ScopedID(az, projectID, created.ID), stateFromBackup(input, created), err
	}
	return ScopedID(az, projectID, ready.ID), stateFromBackup(input, ready), nil
}

func stateFromBackup(input VolumeBackupArgs, backup *nobusapi.Backup) VolumeBackupState {
	state := VolumeBackupState{
		VolumeBackupArgs: input,
		Status:           backup.Status,
		Size:             backup.Size,
		CreatedAt:        backup.CreatedAt,
	}
	if state.Name == "" {
		state.Name = backup.Name
	}
	if state.Description == "" {
		state.Description = backup.Description
	}
	if state.VolumeID == "" {
		state.VolumeID = backup.VolumeID
	}
	if state.Container == "" {
		state.Container = backup.Container
	}
	return state
}

func (*VolumeBackup) Read(ctx context.Context, id string, inputs VolumeBackupArgs, state VolumeBackupState) (string, VolumeBackupArgs, VolumeBackupState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetBackup(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", VolumeBackupArgs{}, VolumeBackupState{}, nil
		}
		return id, inputs, state, err
	}
	args := state.VolumeBackupArgs
	if args.VolumeID == "" {
		args = inputs
	}
	args.ProjectID = projectID
	args.AvailabilityZone = az
	state = stateFromBackup(args, got)
	return ScopedID(az, projectID, got.ID), state.VolumeBackupArgs, state, nil
}

func (*VolumeBackup) Diff(ctx context.Context, id string, olds VolumeBackupState, news VolumeBackupArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("volumeId", news.VolumeID != "" && BareResourceID(olds.VolumeID) != BareResourceID(news.VolumeID), diff)
	replaceDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	replaceDiff("description", olds.Description != news.Description, diff)
	replaceDiff("container", olds.Container != news.Container, diff)
	replaceDiff("incremental", olds.Incremental != news.Incremental, diff)
	replaceDiff("force", olds.Force != news.Force, diff)
	replaceDiff("snapshotId", BareResourceID(olds.SnapshotID) != BareResourceID(news.SnapshotID), diff)
	replaceDiff("metadata", !maps.Equal(olds.Metadata, news.Metadata), diff)
	return diffResponse(diff), nil
}

func (*VolumeBackup) Delete(ctx context.Context, id string, state VolumeBackupState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteBackup(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
