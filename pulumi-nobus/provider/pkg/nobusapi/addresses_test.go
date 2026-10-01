package nobusapi

import (
	"encoding/json"
	"testing"
)

func TestParseAddresses(t *testing.T) {
	const raw = `{
		"demo-net": [
			{"addr": "10.42.0.12", "version": 4, "OS-EXT-IPS:type": "fixed"},
			{"addr": "102.89.1.50", "version": 4, "OS-EXT-IPS:type": "floating"}
		],
		"other": [{"addr": "fd00::1", "version": 6}]
	}`
	var addrs Addresses
	if err := json.Unmarshal([]byte(raw), &addrs); err != nil {
		t.Fatal(err)
	}
	got := ParseAddresses(addrs)
	if got.PrivateIPv4 != "10.42.0.12" {
		t.Fatalf("private = %q", got.PrivateIPv4)
	}
	if got.PublicIPv4 != "102.89.1.50" {
		t.Fatalf("public = %q", got.PublicIPv4)
	}
	if len(got.IPv4) != 2 || len(got.IPv6) != 1 {
		t.Fatalf("ipv4=%v ipv6=%v", got.IPv4, got.IPv6)
	}
	if len(got.Networks) != 2 {
		t.Fatalf("networks=%v", got.Networks)
	}
}

func TestAddressesSingleObject(t *testing.T) {
	const raw = `{"net": {"addr": "10.0.0.2", "version": 4, "OS-EXT-IPS:type": "fixed"}}`
	var addrs Addresses
	if err := json.Unmarshal([]byte(raw), &addrs); err != nil {
		t.Fatal(err)
	}
	got := ParseAddresses(addrs)
	if got.PrivateIPv4 != "10.0.0.2" {
		t.Fatalf("private = %q", got.PrivateIPv4)
	}
}
