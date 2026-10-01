package provider

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

type Volume struct{}

type VolumeArgs struct {
	Size             int               `pulumi:"size"`
	AvailabilityZone string            `pulumi:"availabilityZone"`
	Name             string            `pulumi:"name,optional"`
	Description      string            `pulumi:"description,optional"`
	VolumeType       string            `pulumi:"volumeType,optional"`
	ProjectID        string            `pulumi:"projectId,optional"`
	SnapshotID       string            `pulumi:"snapshotId,optional"`
	SourceVolID      string            `pulumi:"sourceVolId,optional"`
	ImageRef         string            `pulumi:"imageRef,optional"`
	Metadata         map[string]string `pulumi:"metadata,optional"`
	Multiattach      *bool             `pulumi:"multiattach,optional"`
}

type VolumeState struct {
	VolumeArgs
	Status      string `pulumi:"status"`
	Multiattach bool   `pulumi:"isMultiattach"`
	AttachedTo  string `pulumi:"attachedTo"`
	Device      string `pulumi:"device"`
}

func (*Volume) Annotate(a infer.Annotator) {
	a.Describe(&Volume{}, "A Nobus block storage volume.")
}

func (*Volume) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (VolumeArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs, func(a *VolumeArgs) string { return a.AvailabilityZone })
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.Size <= 0 {
		failures = append(failures, p.CheckFailure{Property: "size", Reason: "size must be greater than 0"})
	}
	return args, failures, nil
}

func (*Volume) Create(ctx context.Context, name string, input VolumeArgs, preview bool) (string, VolumeState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", VolumeState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	state := VolumeState{VolumeArgs: input}
	if preview {
		return ScopedID(az, projectID, name), state, nil
	}
	created, err := cfg.Client().CreateVolume(ctx, nobusapi.CreateVolumeRequest{
		Size:             input.Size,
		Name:             input.Name,
		Description:      input.Description,
		VolumeType:       input.VolumeType,
		AvailabilityZone: az,
		SnapshotID:       BareResourceID(input.SnapshotID),
		SourceVolID:      BareResourceID(input.SourceVolID),
		ImageRef:         BareResourceID(input.ImageRef),
		Metadata:         input.Metadata,
		Multiattach:      input.Multiattach,
		ProjectID:        projectID,
	})
	if err != nil {
		return "", VolumeState{}, err
	}
	ready, err := cfg.Client().WaitVolumeStatus(ctx, projectID, az, created.ID, "available")
	if err != nil {
		return ScopedID(az, projectID, created.ID), stateFromVolume(input, created), err
	}
	return ScopedID(az, projectID, ready.ID), stateFromVolume(input, ready), nil
}

func stateFromVolume(input VolumeArgs, vol *nobusapi.Volume) VolumeState {
	state := VolumeState{
		VolumeArgs:  input,
		Status:      vol.Status,
		Multiattach: vol.Multiattach,
	}
	if state.Name == "" {
		state.Name = vol.Name
	}
	if state.Description == "" {
		state.Description = vol.Description
	}
	if state.VolumeType == "" {
		state.VolumeType = vol.VolumeType
	}
	if state.Size == 0 {
		state.Size = vol.Size
	}
	if len(vol.Attachments) > 0 {
		state.AttachedTo = vol.Attachments[0].ServerID
		state.Device = vol.Attachments[0].Device
	}
	return state
}

func (*Volume) Read(ctx context.Context, id string, inputs VolumeArgs, state VolumeState) (string, VolumeArgs, VolumeState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	got, err := cfg.Client().GetVolume(ctx, projectID, az, resourceID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", VolumeArgs{}, VolumeState{}, nil
		}
		return id, inputs, state, err
	}
	args := state.VolumeArgs
	if args.Size == 0 {
		args = inputs
	}
	args.ProjectID = projectID
	args.AvailabilityZone = az
	state = stateFromVolume(args, got)
	return ScopedID(az, projectID, got.ID), state.VolumeArgs, state, nil
}

func (*Volume) Diff(ctx context.Context, id string, olds VolumeState, news VolumeArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("size", news.Size != 0 && olds.Size != news.Size, diff)
	replaceDiff("volumeType", news.VolumeType != "" && olds.VolumeType != news.VolumeType, diff)
	replaceDiff("snapshotId", news.SnapshotID != "" && olds.SnapshotID != news.SnapshotID, diff)
	replaceDiff("sourceVolId", news.SourceVolID != "" && olds.SourceVolID != news.SourceVolID, diff)
	replaceDiff("imageRef", news.ImageRef != "" && olds.ImageRef != news.ImageRef, diff)
	updateDiff("name", news.Name != "" && olds.Name != news.Name, diff)
	updateDiff("description", olds.Description != news.Description, diff)
	updateDiff("metadata", !maps.Equal(olds.Metadata, news.Metadata), diff)
	return diffResponse(diff), nil
}

func (*Volume) Update(ctx context.Context, id string, olds VolumeState, news VolumeArgs, preview bool) (VolumeState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(news.ProjectID, olds.ProjectID), cmp.Or(news.AvailabilityZone, olds.AvailabilityZone))
	if err != nil {
		return olds, err
	}
	news.ProjectID = projectID
	news.AvailabilityZone = az
	state := VolumeState{VolumeArgs: news, Status: olds.Status, Multiattach: olds.Multiattach, AttachedTo: olds.AttachedTo, Device: olds.Device}
	if preview {
		return state, nil
	}
	updated, err := cfg.Client().UpdateVolume(ctx, resourceID, nobusapi.UpdateVolumeRequest{
		Name:             news.Name,
		Description:      news.Description,
		Metadata:         news.Metadata,
		ProjectID:        projectID,
		AvailabilityZone: az,
	})
	if err != nil {
		return state, err
	}
	return stateFromVolume(news, updated), nil
}

func (*Volume) Delete(ctx context.Context, id string, state VolumeState) error {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DeleteVolume(ctx, projectID, az, resourceID)
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}

type VolumeAttachment struct{}

type VolumeAttachmentArgs struct {
	VolumeID         string `pulumi:"volumeId"`
	InstanceID       string `pulumi:"instanceId"`
	AvailabilityZone string `pulumi:"availabilityZone"`
	ProjectID        string `pulumi:"projectId,optional"`
}

type VolumeAttachmentState struct {
	VolumeAttachmentArgs
	Device string `pulumi:"device"`
}

func (*VolumeAttachment) Annotate(a infer.Annotator) {
	a.Describe(&VolumeAttachment{}, "Attaches a Nobus volume to an instance.")
}

func (*VolumeAttachment) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (VolumeAttachmentArgs, []p.CheckFailure, error) {
	args, failures, err := checkResourceArgs(ctx, newInputs, func(a *VolumeAttachmentArgs) string { return a.AvailabilityZone })
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if args.VolumeID == "" {
		failures = append(failures, p.CheckFailure{Property: "volumeId", Reason: "volumeId is required"})
	}
	if args.InstanceID == "" {
		failures = append(failures, p.CheckFailure{Property: "instanceId", Reason: "instanceId is required"})
	}
	return args, failures, nil
}

func (*VolumeAttachment) Create(ctx context.Context, name string, input VolumeAttachmentArgs, preview bool) (string, VolumeAttachmentState, error) {
	cfg := getConfig(ctx)
	projectID, az, err := cfg.ResolveScope(input.ProjectID, input.AvailabilityZone)
	if err != nil {
		return "", VolumeAttachmentState{}, err
	}
	input.ProjectID = projectID
	input.AvailabilityZone = az
	volumeID := BareResourceID(input.VolumeID)
	instanceID := BareResourceID(input.InstanceID)
	input.VolumeID = volumeID
	input.InstanceID = instanceID
	state := VolumeAttachmentState{VolumeAttachmentArgs: input}
	id := ScopedID(az, projectID, volumeID+":"+instanceID)
	if preview {
		return id, state, nil
	}
	if err := cfg.Client().AttachVolume(ctx, nobusapi.AttachVolumeRequest{
		ProjectID:        projectID,
		AvailabilityZone: az,
		ServerID:         instanceID,
		VolumeID:         volumeID,
	}); err != nil {
		return "", VolumeAttachmentState{}, err
	}
	device, err := cfg.Client().WaitVolumeAttached(ctx, projectID, az, volumeID, instanceID)
	if err != nil {
		return id, state, err
	}
	state.Device = device
	return id, state, nil
}

func (*VolumeAttachment) Read(ctx context.Context, id string, inputs VolumeAttachmentArgs, state VolumeAttachmentState) (string, VolumeAttachmentArgs, VolumeAttachmentState, error) {
	cfg := getConfig(ctx)
	az, projectID, resourceID, err := cfg.ResolveImport(id, cmp.Or(state.ProjectID, inputs.ProjectID), cmp.Or(state.AvailabilityZone, inputs.AvailabilityZone))
	if err != nil {
		return id, inputs, state, err
	}
	volumeID := BareResourceID(cmp.Or(state.VolumeID, inputs.VolumeID))
	instanceID := BareResourceID(cmp.Or(state.InstanceID, inputs.InstanceID))
	if volumeID == "" || instanceID == "" {
		volPart, instPart, ok := strings.Cut(resourceID, ":")
		if volumeID == "" {
			volumeID = volPart
		}
		if instanceID == "" && ok {
			instanceID = instPart
		}
	}
	volume, err := cfg.Client().GetVolume(ctx, projectID, az, volumeID)
	if err != nil {
		if errors.Is(err, nobusapi.ErrNotFound) {
			return "", VolumeAttachmentArgs{}, VolumeAttachmentState{}, nil
		}
		return id, inputs, state, err
	}
	for _, attachment := range volume.Attachments {
		if attachment.ServerID == instanceID {
			state.VolumeID = volumeID
			state.InstanceID = instanceID
			state.ProjectID = projectID
			state.AvailabilityZone = az
			state.Device = attachment.Device
			return ScopedID(az, projectID, volumeID+":"+instanceID), state.VolumeAttachmentArgs, state, nil
		}
	}
	return "", VolumeAttachmentArgs{}, VolumeAttachmentState{}, nil
}

func (*VolumeAttachment) Diff(ctx context.Context, id string, olds VolumeAttachmentState, news VolumeAttachmentArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	replaceDiff("availabilityZone", news.AvailabilityZone != "" && olds.AvailabilityZone != news.AvailabilityZone, diff)
	replaceDiff("volumeId", news.VolumeID != "" && BareResourceID(olds.VolumeID) != BareResourceID(news.VolumeID), diff)
	replaceDiff("instanceId", news.InstanceID != "" && BareResourceID(olds.InstanceID) != BareResourceID(news.InstanceID), diff)
	return diffResponse(diff), nil
}

func (*VolumeAttachment) Delete(ctx context.Context, id string, state VolumeAttachmentState) error {
	cfg := getConfig(ctx)
	az, projectID, _, err := cfg.ResolveImport(id, state.ProjectID, state.AvailabilityZone)
	if err != nil {
		return err
	}
	err = cfg.Client().DetachVolume(ctx, nobusapi.AttachVolumeRequest{
		ProjectID:        projectID,
		AvailabilityZone: az,
		ServerID:         BareResourceID(state.InstanceID),
		VolumeID:         BareResourceID(state.VolumeID),
	})
	if errors.Is(err, nobusapi.ErrNotFound) {
		return nil
	}
	return err
}
