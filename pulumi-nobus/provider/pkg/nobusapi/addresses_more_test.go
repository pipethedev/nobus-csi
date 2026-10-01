package nobusapi_test

import (
	"encoding/json"
	"testing"

	"github.com/brimble/pulumi-nobus/provider/pkg/nobusapi"
)

func TestParseAddressesEdgeCases(t *testing.T) {
	if got := nobusapi.ParseAddresses(nil); got.PrivateIPv4 != "" {
		t.Fatalf("%#v", got)
	}
	got := nobusapi.ParseAddresses(nobusapi.Addresses{
		"only": {{Addr: "10.0.0.1", Version: 4}},
	})
	if got.PrivateIPv4 != "10.0.0.1" {
		t.Fatalf("%#v", got)
	}
	got = nobusapi.ParseAddresses(nobusapi.Addresses{
		"n": {
			{Addr: "10.0.0.1", Version: 4},
			{Addr: "10.0.0.2", Version: 4},
		},
	})
	if got.PublicIPv4 != "10.0.0.2" {
		t.Fatalf("fallback public %#v", got)
	}
}

func TestAddressesUnmarshalNullAndEmpty(t *testing.T) {
	var a nobusapi.Addresses
	if err := json.Unmarshal([]byte("null"), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte("{}"), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"net": null}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"net": []}`), &a); err != nil {
		t.Fatal(err)
	}
}
