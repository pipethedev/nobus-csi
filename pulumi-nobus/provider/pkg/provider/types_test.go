package provider

import "testing"

func TestParseScopedID(t *testing.T) {
	az, project, id := ParseScopedID("ng1/proj/abc")
	if az != "ng1" || project != "proj" || id != "abc" {
		t.Fatalf("got %q %q %q", az, project, id)
	}
	if BareResourceID("ng1/proj/abc") != "abc" {
		t.Fatal("BareResourceID")
	}
	if BareResourceID("abc") != "abc" {
		t.Fatal("bare passthrough")
	}
}
