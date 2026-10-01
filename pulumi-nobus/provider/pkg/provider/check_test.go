package provider

import "testing"

func TestStringSlicesEqual(t *testing.T) {
	if !stringSlicesEqual([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("order-insensitive equal")
	}
	if stringSlicesEqual([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("length mismatch")
	}
	if !stringSlicesEqual([]string{"ng1/p/sg1"}, []string{"sg1"}) {
		t.Fatal("bare id compare")
	}
}

func TestSliceDiff(t *testing.T) {
	add, remove := sliceDiff([]string{"a", "b"}, []string{"b", "c"})
	if len(add) != 1 || add[0] != "c" {
		t.Fatalf("add=%v", add)
	}
	if len(remove) != 1 || remove[0] != "a" {
		t.Fatalf("remove=%v", remove)
	}
}
