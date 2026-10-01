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

type VolumeSnapshot struct{}

type VolumeSnapshotArgs struct {
	VolumeID         string            `pulumi:"volumeId"`
	AvailabilityZone string            `pulumi:"availabilityZone"`
	Name             string            `pulumi:"name,optional"`
	Description      string            `pulumi:"description,optional"`
	Force            bool              `pulumi:"force,optional"`
	Metadata         map[string]string `pulumi:"metadata,optional"`
	ProjectID        string            `pulumi:"projectId,optional"`
}

type VolumeSnapshotState struct {
	VolumeSnapshotArgs
	Status    string `pulumi:"status"`
	Size      int    `pulumi:"size"`
	CreatedAt string `pulumi:"createdAt"`
}

func (*VolumeSnapshot) Annotate(a infer.Annotator) {
	a.Describe(&VolumeSnapshot{}, "A point-in-time snapshot of a Nobus volume.")
}

func (*VolumeSnapshot) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (VolumeSnapshotArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs, func(a *VolumeSnapshotArgs) string { return a.AvailabilityZone })
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.VolumeID == "" {
		failures = append(failures, p.CheckFailure{Property: "volumeId", Reason: "volumeId is required"})
	}
	return args, failures, nil
}

func (*VolumeSnapshot) Create(ctx context.Context, name string, input VolumeSnapshotArgs, preview bool) (string, VolumeSnapshotState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", VolumeSnapshotState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	input.VolumeID = BareResourceID(input.VolumeID)
	state := VolumeSnapshotState{VolumeSnapshotArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateSnapshot(ctx, nobusapi.CreateSnapshotRequest{
		VolumeID:         input.VolumeID,
		Name:             input.Name,
		Description:      input.Description,
		Force:            input.Force,
		Metadata:         input.Metadata,
		ProjectID:        projectID,
		AvailabilityZone: az,
	})
	if err != nil {
		return "", VolumeSnapshotState{}, err
	}
	ready, err := cfg.Client().WaitSnapshotStatus(ctx, projectID, az, created.ID, "available")
	if err != nil {
		return ScopedID(az, projectID, created.ID), stateFromSnapshot(input, created), err
	}
	return ScopedID(az, projectID, ready.ID), stateFromSnapshot(input, ready), nil
}

func stateFromSnapshot(input VolumeSnapshotArgs, snap *nobusapi.Snapshot) VolumeSnapshotState {
	state := VolumeSnapshotState{
		VolumeSnapshotArgs: input,
		Status:             snap.Status,
		Size:               snap.Size,
		CreatedAt:          snap.CreatedAt,
	}
	if state.Name == "" {
		state.Name = snap.Name
	}
	if state.Description == "" {
		state.Description = snap.Description
	}
	if state.VolumeID == "" {
		state.VolumeID = snap.VolumeID
	}
	return state
}

func (*VolumeSnapshot) Read(ctx context.Context, id string, inputs VolumeSnapshotArgs, state VolumeSnapshotState) (string, VolumeSnapshotArgs, VolumeSnapshotState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetSnapshot(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", VolumeSnapshotArgs{}, VolumeSnapshotState{}, nil
		}
		return id, inputs, state, err
	}
	args := state.VolumeSnapshotArgs
	if args.VolumeID == "" {
		args = inputs
	}
	args.ProjectID = projectID
	args.AvailabilityZone = az
	state = stateFromSnapshot(args, got)
	return ScopedID(az, projectID, got.ID), state.VolumeSnapshotArgs, state, nil
}

func (*VolumeSnapshot) Diff(ctx context.Context, id string, olds VolumeSnapshotState, news VolumeSnapshotArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("volumeId", news.VolumeID != "" && BareResourceID(olds.VolumeID) != BareResourceID(news.VolumeID), diff)
	replaceDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	replaceDiff("description", olds.Description != news.Description, diff)
	replaceDiff("force", olds.Force != news.Force, diff)
	replaceDiff("metadata", !maps.Equal(olds.Metadata, news.Metadata), diff)
	return diffResponse(diff), nil
}

func (*VolumeSnapshot) Delete(ctx context.Context, id string, state VolumeSnapshotState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteSnapshot(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
