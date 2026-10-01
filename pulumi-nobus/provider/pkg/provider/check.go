package provider

import (
	"context"
	"sort"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// checkResourceArgs fills provider defaults for a blank availability zone or
// project, then rejects a resource that still has no availability zone.
func checkResourceArgs[I any](
	ctx context.Context,
	newInputs resource.PropertyMap,
	availabilityZone func(*I) string,
) (I, []p.CheckFailure, error) {
	cfg := getConfig(ctx)
	inputs := newInputs.Copy()
	if cfg != nil {
		if blankInput(inputs, "availabilityZone") && cfg.AvailabilityZone != "" {
			inputs["availabilityZone"] = resource.NewStringProperty(cfg.AvailabilityZone)
		}
		if blankInput(inputs, "projectId") && cfg.ProjectID != "" {
			inputs["projectId"] = resource.NewStringProperty(cfg.ProjectID)
		}
	}

	args, failures, err := infer.DefaultCheck[I](ctx, inputs)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}
	if strings.TrimSpace(availabilityZone(&args)) == "" {
		failures = append(failures, p.CheckFailure{
			Property: "availabilityZone",
			Reason:   "availabilityZone is required (set on the resource or via provider config / NOBUS_AVAILABILITY_ZONE)",
		})
	}
	return args, failures, nil
}

func blankInput(inputs resource.PropertyMap, key string) bool {
	value, ok := inputs[resource.PropertyKey(key)]
	if !ok || value.IsNull() {
		return true
	}
	return value.IsString() && strings.TrimSpace(value.StringValue()) == ""
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if BareResourceID(aa[i]) != BareResourceID(bb[i]) {
			return false
		}
	}
	return true
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func sliceDiff(old, news []string) (add, remove []string) {
	oldSet := map[string]struct{}{}
	newSet := map[string]struct{}{}
	for _, v := range old {
		oldSet[BareResourceID(v)] = struct{}{}
	}
	for _, v := range news {
		newSet[BareResourceID(v)] = struct{}{}
	}
	for v := range newSet {
		if _, ok := oldSet[v]; !ok {
			add = append(add, v)
		}
	}
	for v := range oldSet {
		if _, ok := newSet[v]; !ok {
			remove = append(remove, v)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}
