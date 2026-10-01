package nobusapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type AddressEntry struct {
	Addr    string `json:"addr"`
	Version int    `json:"version"`
	Type    string `json:"OS-EXT-IPS:type"`
}

// Addresses is the OpenStack address map: network name to IP assignments.
// A network value may be an array or a single object.
type Addresses map[string][]AddressEntry

func (a *Addresses) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*a = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode instance addresses: %w", err)
	}
	if len(raw) == 0 {
		*a = Addresses{}
		return nil
	}
	out := make(Addresses, len(raw))
	for name, payload := range raw {
		entries, err := decodeAddressEntries(payload)
		if err != nil {
			return fmt.Errorf("decode addresses on %s: %w", name, err)
		}
		out[name] = entries
	}
	*a = out
	return nil
}

func decodeAddressEntries(payload []byte) ([]AddressEntry, error) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		return nil, nil
	}
	if payload[0] == '[' {
		var entries []AddressEntry
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	var one AddressEntry
	if err := json.Unmarshal(payload, &one); err != nil {
		return nil, err
	}
	if one.Addr == "" {
		return nil, fmt.Errorf("%w: address entry missing addr", ErrInvalid)
	}
	return []AddressEntry{one}, nil
}

type ParsedAddresses struct {
	Networks    []string
	IPv4        []string
	IPv6        []string
	PrivateIPv4 string
	PublicIPv4  string
}

func ParseAddresses(raw Addresses) ParsedAddresses {
	if len(raw) == 0 {
		return ParsedAddresses{}
	}
	networks := make([]string, 0, len(raw))
	for name := range raw {
		networks = append(networks, name)
	}
	sort.Strings(networks)

	out := ParsedAddresses{Networks: networks}
	for _, name := range networks {
		for _, entry := range raw[name] {
			switch entry.Version {
			case 6:
				out.IPv6 = append(out.IPv6, entry.Addr)
			default:
				out.IPv4 = append(out.IPv4, entry.Addr)
				if entry.Type == "floating" || entry.Type == "public" {
					if out.PublicIPv4 == "" {
						out.PublicIPv4 = entry.Addr
					}
				} else if out.PrivateIPv4 == "" {
					out.PrivateIPv4 = entry.Addr
				}
			}
		}
	}
	if out.PrivateIPv4 == "" && len(out.IPv4) > 0 {
		out.PrivateIPv4 = out.IPv4[0]
	}
	if out.PublicIPv4 == "" && len(out.IPv4) > 1 {
		out.PublicIPv4 = out.IPv4[len(out.IPv4)-1]
	}
	return out
}
