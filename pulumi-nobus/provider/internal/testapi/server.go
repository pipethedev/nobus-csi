package testapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func WriteJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encode json: %v", err)
	}
}

func OkData(data any) map[string]any {
	return map[string]any{"status": true, "data": data}
}

// Mock is a stateful httptest handler covering Nobus v3 paths used by the provider.
type Mock struct {
	t *testing.T
	mu sync.Mutex

	instances map[string]map[string]any
	volumes   map[string]map[string]any
	snapshots map[string]map[string]any
	backups   map[string]map[string]any
	keypairs  map[string]map[string]any
	networks  map[string]map[string]any
	subnets   map[string]map[string]any
	ports     map[string]map[string]any
	routers   map[string]map[string]any
	sgs       map[string]map[string]any
	sgRules   map[string]map[string]any
	fips      map[string]map[string]any
	flavors   []map[string]any
	images    []map[string]any
	projects  []map[string]any
	zones     []map[string]any

	FailNext map[string]int // path prefix -> status code once
	Calls    []string
}

func New(t *testing.T) *Mock {
	t.Helper()
	m := &Mock{
		t:         t,
		instances: map[string]map[string]any{},
		volumes:   map[string]map[string]any{},
		snapshots: map[string]map[string]any{},
		backups:   map[string]map[string]any{},
		keypairs:  map[string]map[string]any{},
		networks:  map[string]map[string]any{},
		subnets:   map[string]map[string]any{},
		ports:     map[string]map[string]any{},
		routers:   map[string]map[string]any{},
		sgs:       map[string]map[string]any{},
		sgRules:   map[string]map[string]any{},
		fips:      map[string]map[string]any{},
		flavors: []map[string]any{
			{"id": "fl-1", "name": "s1.small", "vcpus": 1, "ram": 1024, "disk": 20},
			{"id": "fl-2", "name": "s1.medium", "vcpus": 2, "ram": 2048, "disk": 40},
		},
		images: []map[string]any{
			{"id": "img-1", "name": "Ubuntu"},
			{"id": "img-2", "name": "Debian"},
		},
		projects: []map[string]any{
			{"id": "proj-1", "name": "default", "availability_zone": "ng1", "is_default": true},
			{"id": "proj-2", "name": "other", "availability_zone": "ng1", "is_default": false},
		},
		zones: []map[string]any{
			{"name": "ng1"},
			{"name": "nobus-wa-az1"},
		},
		FailNext: map[string]int{},
	}
	return m
}

func (m *Mock) SeedInstance(id string, item map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item["id"] == nil {
		item["id"] = id
	}
	if item["status"] == nil {
		item["status"] = "ACTIVE"
	}
	m.instances[id] = item
}

func (m *Mock) SeedVolume(id string, item map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item["id"] == nil {
		item["id"] = id
	}
	if item["status"] == nil {
		item["status"] = "available"
	}
	if item["attachments"] == nil {
		item["attachments"] = []any{}
	}
	m.volumes[id] = item
}

func (m *Mock) SeedKeyPair(id string, item map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item["id"] == nil {
		item["id"] = id
	}
	m.keypairs[id] = item
}

func (m *Mock) SeedNetwork(id string, item map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item["id"] == nil {
		item["id"] = id
	}
	m.networks[id] = item
}

func (m *Mock) Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(m.serve))
}

func (m *Mock) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, r.Method+" "+r.URL.Path)

	for prefix, code := range m.FailNext {
		if strings.HasPrefix(r.URL.Path, prefix) {
			delete(m.FailNext, prefix)
			WriteJSON(m.t, w, code, map[string]any{"message": http.StatusText(code)})
			return
		}
	}

	switch {
	case r.URL.Path == "/api/v3/keypair/" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "kp-" + body["name"].(string)
		item := map[string]any{
			"id": id, "name": body["name"], "public_key": "ssh-rsa A", "private_key": "PRIV",
			"fingerprint": "aa:bb", "type": "ssh",
		}
		m.keypairs[id] = item
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/keypair/" && r.Method == http.MethodGet:
		items := make([]map[string]any, 0, len(m.keypairs))
		for _, v := range m.keypairs {
			items = append(items, v)
		}
		WriteJSON(m.t, w, 200, OkData(items))
	case r.URL.Path == "/api/v3/keypair/" && r.Method == http.MethodDelete:
		id := r.URL.Query().Get("keypair_id")
		delete(m.keypairs, id)
		w.WriteHeader(204)

	case r.URL.Path == "/api/v3/instance" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "srv-1"
		item := map[string]any{
			"id": id, "name": body["name"], "status": "ACTIVE", "key_name": body["key_name"],
			"created": "now", "updated": "now",
			"addresses": map[string]any{
				"net": []map[string]any{
					{"addr": "10.0.0.5", "version": 4, "OS-EXT-IPS:type": "fixed"},
					{"addr": "1.2.3.4", "version": 4, "OS-EXT-IPS:type": "floating"},
				},
			},
		}
		m.instances[id] = item
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/instance" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("server_id")
		item, ok := m.instances[id]
		if !ok {
			WriteJSON(m.t, w, 404, map[string]any{"message": "not found"})
			return
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/instance" && r.Method == http.MethodDelete:
		delete(m.instances, r.URL.Query().Get("server_id"))
		w.WriteHeader(204)
	case r.URL.Path == "/api/v3/instance/list":
		items := make([]map[string]any, 0, len(m.instances))
		for _, v := range m.instances {
			items = append(items, v)
		}
		WriteJSON(m.t, w, 200, OkData(items))
	case r.URL.Path == "/api/v3/instance/update-server":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["server_id"].(string)
		if item, ok := m.instances[id]; ok {
			if name, ok := body["name"].(string); ok && name != "" {
				item["name"] = name
			}
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))
	case r.URL.Path == "/api/v3/instance/resize":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["server_id"].(string)
		if item, ok := m.instances[id]; ok {
			item["status"] = "VERIFY_RESIZE"
			item["flavor"] = body["flavor_id"]
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))
	case r.URL.Path == "/api/v3/instance/confirm-resize":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["server_id"].(string)
		if item, ok := m.instances[id]; ok {
			item["status"] = "ACTIVE"
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))
	case r.URL.Path == "/api/v3/instance/attach-security-group",
		r.URL.Path == "/api/v3/instance/remove-security-group",
		r.URL.Path == "/api/v3/instance/set-tags":
		_, _ = io.Copy(io.Discard, r.Body)
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))

	case r.URL.Path == "/api/v3/volume/" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "vol-1"
		item := map[string]any{
			"id": id, "name": body["name"], "size": body["size"], "status": "available",
			"volume_type": body["volume_type"], "availability_zone": body["availability_zone"],
			"attachments": []any{}, "metadata": body["metadata"],
		}
		m.volumes[id] = item
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/list":
		items := make([]map[string]any, 0, len(m.volumes))
		for _, v := range m.volumes {
			items = append(items, v)
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"items": items}))
	case r.URL.Path == "/api/v3/volume/" && r.Method == http.MethodPut:
		id := r.URL.Query().Get("volume_id")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		item := m.volumes[id]
		if item == nil {
			WriteJSON(m.t, w, 404, map[string]any{"message": "not found"})
			return
		}
		if name, ok := body["name"].(string); ok {
			item["name"] = name
		}
		if desc, ok := body["description"].(string); ok {
			item["description"] = desc
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/" && r.Method == http.MethodDelete:
		delete(m.volumes, r.URL.Query().Get("volume_id"))
		w.WriteHeader(204)
	case r.URL.Path == "/api/v3/volume/attach":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vid, _ := body["volume_id"].(string)
		sid, _ := body["server_id"].(string)
		if item, ok := m.volumes[vid]; ok {
			item["status"] = "in-use"
			item["attachments"] = []map[string]any{{"server_id": sid, "device": "/dev/vdb"}}
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))
	case r.URL.Path == "/api/v3/volume/detach":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vid, _ := body["volume_id"].(string)
		if item, ok := m.volumes[vid]; ok {
			item["status"] = "available"
			item["attachments"] = []any{}
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"message": "ok"}))

	case r.URL.Path == "/api/v3/volume/snapshot" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "snap-1"
		item := map[string]any{
			"id": id, "name": body["name"], "description": body["description"],
			"volume_id": body["volume_id"], "status": "available", "size": 10,
			"created_at": "now", "metadata": body["metadata"],
		}
		m.snapshots[id] = item
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/snapshot" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("snapshot_id")
		item, ok := m.snapshots[id]
		if !ok {
			WriteJSON(m.t, w, 404, map[string]any{"message": "not found"})
			return
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/snapshot" && r.Method == http.MethodDelete:
		delete(m.snapshots, r.URL.Query().Get("snapshot_id"))
		w.WriteHeader(204)
	case r.URL.Path == "/api/v3/volume/snapshot/list":
		items := make([]map[string]any, 0, len(m.snapshots))
		for _, v := range m.snapshots {
			items = append(items, v)
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"items": items}))

	case r.URL.Path == "/api/v3/volume/backup" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "bak-1"
		item := map[string]any{
			"id": id, "name": body["name"], "description": body["description"],
			"volume_id": body["volume_id"], "status": "available", "size": 10,
			"container": body["container"], "created_at": "now",
			"availability_zone": body["availability_zone"], "metadata": body["metadata"],
		}
		m.backups[id] = item
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/backup" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("backup_id")
		item, ok := m.backups[id]
		if !ok {
			WriteJSON(m.t, w, 404, map[string]any{"message": "not found"})
			return
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/volume/backup" && r.Method == http.MethodDelete:
		delete(m.backups, r.URL.Query().Get("backup_id"))
		w.WriteHeader(204)
	case r.URL.Path == "/api/v3/volume/backup/list":
		items := make([]map[string]any, 0, len(m.backups))
		for _, v := range m.backups {
			items = append(items, v)
		}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"items": items}))

	case r.URL.Path == "/api/v3/volume/availability_zone/list":
		WriteJSON(m.t, w, 200, OkData(map[string]any{"items": m.zones}))

	case r.URL.Path == "/api/v3/network/" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		nid, sid, rid := "net-1", "sub-1", "rtr-1"
		m.networks[nid] = map[string]any{
			"id": nid, "name": body["network_name"], "status": "ACTIVE", "admin_state_up": true,
			"mtu": 1500, "shared": false, "port_security_enabled": true, "subnets": []string{sid},
		}
		m.subnets[sid] = map[string]any{"id": sid, "network_id": nid, "cidr": body["subnet_cidr"], "ip_version": 4}
		m.routers[rid] = map[string]any{"id": rid, "name": body["router_name"], "status": "ACTIVE"}
		WriteJSON(m.t, w, 200, OkData(map[string]any{"network_id": nid, "subnet_id": sid, "router_id": rid}))
	case r.URL.Path == "/api/v3/network/" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("network_id")
		item, ok := m.networks[id]
		if !ok {
			WriteJSON(m.t, w, 404, map[string]any{"message": "not found"})
			return
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/network/" && r.Method == http.MethodPut:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["network_id"].(string)
		item := m.networks[id]
		if net, ok := body["network"].(map[string]any); ok && item != nil {
			for k, v := range net {
				item[k] = v
			}
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case r.URL.Path == "/api/v3/network/" && r.Method == http.MethodDelete:
		delete(m.networks, r.URL.Query().Get("network_id"))
		w.WriteHeader(204)

	case strings.HasPrefix(r.URL.Path, "/api/v3/network/subnet"):
		m.handleGenericCRUD(w, r, "subnet", "subnet_id", m.subnets, map[string]any{
			"id": "sub-x", "network_id": "net-1", "cidr": "10.0.0.0/24", "ip_version": 4, "name": "s",
		})
	case strings.HasPrefix(r.URL.Path, "/api/v3/network/port"):
		m.handleGenericCRUD(w, r, "port", "port_id", m.ports, map[string]any{
			"id": "port-1", "network_id": "net-1", "status": "ACTIVE", "mac_address": "aa:bb",
		})
	case strings.HasPrefix(r.URL.Path, "/api/v3/network/router"):
		m.handleGenericCRUD(w, r, "router", "router_id", m.routers, map[string]any{
			"id": "rtr-x", "name": "r", "status": "ACTIVE",
		})
	case strings.HasPrefix(r.URL.Path, "/api/v3/network/security-group-rule"):
		m.handleGenericCRUD(w, r, "security_group_rule", "security_group_rule_id", m.sgRules, map[string]any{
			"id": "rule-1", "security_group_id": "sg-1", "direction": "ingress", "ethertype": "IPv4",
		})
	case strings.HasPrefix(r.URL.Path, "/api/v3/network/security-group"):
		m.handleGenericCRUD(w, r, "security_group", "security_group_id", m.sgs, map[string]any{
			"id": "sg-1", "name": "web", "description": "d",
		})
	case strings.HasPrefix(r.URL.Path, "/api/v3/network/floating-ip"):
		m.handleGenericCRUD(w, r, "floatingip", "floating_ip_id", m.fips, map[string]any{
			"id": "fip-1", "floating_network_id": "ext", "floating_ip_address": "8.8.8.8", "status": "DOWN",
		})

	case r.URL.Path == "/api/v3/flavor/":
		WriteJSON(m.t, w, 200, OkData(m.flavors))
	case r.URL.Path == "/api/v3/image/":
		WriteJSON(m.t, w, 200, OkData(m.images))
	case r.URL.Path == "/api/v3/project/":
		WriteJSON(m.t, w, 200, OkData(m.projects))

	default:
		WriteJSON(m.t, w, 404, map[string]any{"message": "unhandled " + r.Method + " " + r.URL.Path})
	}
}

func (m *Mock) handleGenericCRUD(w http.ResponseWriter, r *http.Request, wrapKey, idQuery string, store map[string]map[string]any, template map[string]any) {
	switch r.Method {
	case http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		item := map[string]any{}
		for k, v := range template {
			item[k] = v
		}
		if nested, ok := body[wrapKey].(map[string]any); ok {
			for k, v := range nested {
				item[k] = v
			}
		}
		// floatingip create uses floatingip key and may omit id in nested
		if id, _ := item["id"].(string); id != "" {
			store[id] = item
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case http.MethodGet:
		id := r.URL.Query().Get(idQuery)
		// securitygroup_id vs security_group_id quirks
		if id == "" {
			id = r.URL.Query().Get("securitygroup_id")
		}
		item, ok := store[id]
		if !ok {
			// return template-shaped 404 for missing
			WriteJSON(m.t, w, 404, map[string]any{"message": id + " not found"})
			return
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case http.MethodPut:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := ""
		for _, k := range []string{idQuery, "securitygroup_id", "floatingip_id", "subnet_id", "port_id", "router_id"} {
			if v, ok := body[k].(string); ok && v != "" {
				id = v
				break
			}
		}
		if id == "" {
			id = r.URL.Query().Get(idQuery)
		}
		item := store[id]
		if item == nil {
			item = map[string]any{}
			for k, v := range template {
				item[k] = v
			}
			item["id"] = id
			store[id] = item
		}
		if nested, ok := body[wrapKey].(map[string]any); ok {
			for k, v := range nested {
				item[k] = v
			}
		}
		WriteJSON(m.t, w, 200, OkData(item))
	case http.MethodDelete:
		id := r.URL.Query().Get(idQuery)
		delete(store, id)
		w.WriteHeader(204)
	default:
		WriteJSON(m.t, w, 405, map[string]any{"message": "method not allowed"})
	}
}
