package provider

import (
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
)

// Known Nobus availability zones. Use these constants or any valid AZ slug returned by the API.
const (
	AvailabilityZoneNG1        = "ng1"
	AvailabilityZoneNobusWAAZ1 = "nobus-wa-az1"
)

// Common Nobus flavor names. Resolve them with getFlavor at runtime.
const (
	FlavorS1Small  = "s1.small"
	FlavorS1Medium = "s1.medium"
	FlavorS1Large  = "s1.large"
)

// ScopedID encodes location into a Pulumi import ID:
//
//	{availabilityZone}/{projectId}/{resourceId}
func ScopedID(availabilityZone, projectID, resourceID string) string {
	return availabilityZone + "/" + projectID + "/" + resourceID
}

// ParseScopedID parses an import/resource ID of the form az/projectId/id.
// If the ID has no slashes, resourceID is returned alone (caller must supply AZ/project).
func ParseScopedID(id string) (availabilityZone, projectID, resourceID string) {
	parts := strings.Split(id, "/")
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return parts[0], "", parts[1]
	default:
		return "", "", id
	}
}

// BareResourceID returns the raw API id from a scoped or bare Pulumi id.
func BareResourceID(id string) string {
	_, _, resourceID := ParseScopedID(id)
	return resourceID
}

func bareIDs(ids []string) []string {
	if len(ids) == 0 {
		return ids
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = BareResourceID(id)
	}
	return out
}

func replaceDiff(field string, changed bool, out map[string]p.PropertyDiff) {
	if changed {
		out[field] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
}

func updateDiff(field string, changed bool, out map[string]p.PropertyDiff) {
	if changed {
		out[field] = p.PropertyDiff{Kind: p.Update}
	}
}

func diffResponse(diff map[string]p.PropertyDiff) p.DiffResponse {
	return p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}
}

func requireAZ(az string) error {
	if strings.TrimSpace(az) == "" {
		return fmt.Errorf("availabilityZone is required on the resource (Nobus has multiple AZs; set it explicitly like DigitalOcean region)")
	}
	return nil
}
