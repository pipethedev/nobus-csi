package cloud

import (
	"context"
	"testing"
)

func TestFake_CreateVolume_DoesNotReuseDeletedIDs(t *testing.T) {
	fake := NewFake(InstanceMetadata{InstanceID: "server-1"})
	first, err := fake.CreateVolume(context.Background(), VolumeSpec{Name: "one", SizeBytes: bytesPerGiB, AvailabilityZone: "az1"})
	if err != nil {
		t.Fatalf("create first volume: %v", err)
	}
	if err := fake.DeleteVolume(context.Background(), first.ID); err != nil {
		t.Fatalf("delete first volume: %v", err)
	}
	second, err := fake.CreateVolume(context.Background(), VolumeSpec{Name: "two", SizeBytes: bytesPerGiB, AvailabilityZone: "az1"})
	if err != nil {
		t.Fatalf("create second volume: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected unique volume id after delete, reused %q", second.ID)
	}
}

func TestFake_ListVolumes_PaginatesSortedIDs(t *testing.T) {
	fake := NewFake(InstanceMetadata{InstanceID: "server-1"})
	for _, name := range []string{"c", "a", "b"} {
		if _, err := fake.CreateVolume(context.Background(), VolumeSpec{Name: name, SizeBytes: bytesPerGiB, AvailabilityZone: "az1"}); err != nil {
			t.Fatalf("create volume %s: %v", name, err)
		}
	}
	first, token, err := fake.ListVolumes(context.Background(), Page{Size: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(first) != 2 || first[0].ID != "vol-1" || first[1].ID != "vol-2" {
		t.Fatalf("expected sorted first page, got %+v", first)
	}
	second, token, err := fake.ListVolumes(context.Background(), Page{Size: 2, Token: token})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(second) != 1 || second[0].ID != "vol-3" || token != "" {
		t.Fatalf("expected remaining vol-3, got %+v token %q", second, token)
	}
}
