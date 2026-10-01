---
title: Nobus Installation & Configuration
meta_desc: Install the Nobus Pulumi provider and configure API credentials and availability zones.
layout: package
---

## Installation

Once published, install from the Pulumi Registry / package managers:

```bash
# Go
go get github.com/brimble/pulumi-nobus/sdk/go/nobus@latest

# Node.js (after npm publish)
npm install @brimble/nobus

# Python (after PyPI publish)
pip install brimble-nobus
```

Or pin a local / debug plugin while developing:

```bash
make build install
# or:
export PULUMI_DEBUG_PROVIDERS=nobus=$PWD/bin/pulumi-resource-nobus
```

## Configuration

| Pulumi config | Environment variable | Required | Notes |
|---------------|----------------------|----------|-------|
| `nobus:apiKey` | `NOBUS_API_KEY` | Yes | Bearer API key (secret) |
| `nobus:endpoint` | `NOBUS_ENDPOINT` | No | Default `https://cloud-api.nobus.io` |
| `nobus:projectId` | `NOBUS_PROJECT_ID` | Recommended | Default project for resources that omit `projectId` |
| `nobus:availabilityZone` | `NOBUS_AVAILABILITY_ZONE` | Recommended | Fallback only — prefer setting `availabilityZone` on each resource |

```bash
pulumi config set --secret nobus:apiKey "$NOBUS_API_KEY"
pulumi config set nobus:projectId "$NOBUS_PROJECT_ID"
```

Mint an API key from the Nobus dashboard or via the login/OTP flow documented in the CSI repo.

## Availability zones

Nobus is multi-AZ. **`availabilityZone` is required on every resource** (and on `getFlavor` / `getImage`), similar to DigitalOcean `region`.

Known helpers in the Go SDK:

- `AvailabilityZoneNG1` (`ng1`)
- `AvailabilityZoneNobusWAAZ1` (`nobus-wa-az1`)
- Flavors: `FlavorS1Small`, `FlavorS1Medium`, `FlavorS1Large`

Changing AZ (or other ForceNew fields such as instance image) replaces the resource. Instance **name**, **flavor** (resize), **securityGroups**, and **tags** update in place.

## Import

Resource IDs are scoped:

```text
{availabilityZone}/{projectId}/{resourceId}
```

```bash
pulumi import nobus:index:Instance demo ng1/PROJECT_UUID/SERVER_UUID
```

Volume attachments use `az/projectId/volumeId:instanceId`.

## Resources

| Type | Purpose |
|------|---------|
| `nobus.KeyPair` | SSH key pair |
| `nobus.Instance` | Compute server |
| `nobus.Volume` / `VolumeAttachment` | Block storage |
| `nobus.Network` / `Subnet` / `Port` / `Router` | Networking |
| `nobus.SecurityGroup` / `SecurityGroupRule` | Firewall |
| `nobus.FloatingIp` | Floating IP |

Invokes: `getFlavor`, `getImage`, `getProject`.
