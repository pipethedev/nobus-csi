package provider

import (
	"context"
	"sort"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// checkResourceArgs runs DefaultCheck then ensures availabilityZone is set
// (from the input or provider default).
func checkResourceArgs[I any](
	ctx context.Context,
	newInputs resource.PropertyMap,
	getAZ func(*I) *string,
	getProject func(*I) *string,
) (I, []p.CheckFailure, error) {
	args, failures, err := infer.DefaultCheck[I](ctx, newInputs)
	if err != nil || len(failures) > 0 {
		return args, failures, err
	}

	cfg := getConfig(ctx)
	azPtr := getAZ(&args)
	if azPtr != nil && strings.TrimSpace(*azPtr) == "" && cfg != nil && cfg.AvailabilityZone != "" {
		*azPtr = cfg.AvailabilityZone
	}
	if azPtr != nil && strings.TrimSpace(*azPtr) == "" {
		failures = append(failures, p.CheckFailure{
			Property: "availabilityZone",
			Reason:   "availabilityZone is required (set on the resource or via provider config / NOBUS_AVAILABILITY_ZONE)",
		})
	}

	if getProject != nil {
		projPtr := getProject(&args)
		if projPtr != nil && strings.TrimSpace(*projPtr) == "" && cfg != nil && cfg.ProjectID != "" {
			*projPtr = cfg.ProjectID
		}
	}

	return args, failures, nil
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
