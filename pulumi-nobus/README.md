# Pulumi Nobus Provider

Native Go Pulumi provider for [Nobus Cloud](https://cloud.nobus.io) covering servers, SSH key pairs, volumes, and networking.

API reference: https://cloud-api.nobus.io/api/v3/openapi.json

## Resources

| Type | Purpose |
|------|---------|
| `nobus.KeyPair` | SSH key pair |
| `nobus.Instance` | Compute server |
| `nobus.Volume` | Block volume |
| `nobus.VolumeAttachment` | Attach volume to instance |
| `nobus.VolumeSnapshot` | Point-in-time volume snapshot |
| `nobus.VolumeBackup` | Volume backup |
| `nobus.Network` | Network + subnet + router bundle |
| `nobus.Subnet` / `Port` / `Router` | Fine-grained networking |
| `nobus.SecurityGroup` / `SecurityGroupRule` | Firewall rules |
| `nobus.FloatingIp` | Public floating IP |

Invokes: `GetFlavor`, `GetImage`, `GetProject`, `GetAvailabilityZones`.

## Availability zones

Nobus is multi-AZ. Like DigitalOcean `region`, **`availabilityZone` is required on every resource** (and on `GetFlavor` / `GetImage`). Provider config `availabilityZone` is only a convenience default when a resource omits it at runtime resolution — prefer setting it explicitly.

Helpers (also exported from the Go SDK after `make sdk`):

- Zones: `AvailabilityZoneNG1`, `AvailabilityZoneNobusWAAZ1`
- Flavors: `FlavorS1Small`, `FlavorS1Medium`, `FlavorS1Large`

Changing AZ (or other ForceNew fields such as instance image) replaces the resource. Instance **name**, **flavor** (resize + confirm), **securityGroups**, and **tags** update in place.

## Registry docs

Pulumi Registry package pages live under [`docs/`](./docs/):

- [`docs/_index.md`](./docs/_index.md) — package overview + multi-language samples
- [`docs/installation-configuration.md`](./docs/installation-configuration.md) — install + credentials
- [`docs/REGISTRY.md`](./docs/REGISTRY.md) — checklist for listing on the community registry after the first release

## Provider config

| Config | Env | Notes |
|--------|-----|-------|
| `apiKey` | `NOBUS_API_KEY` | Required (Bearer) |
| `endpoint` | `NOBUS_ENDPOINT` | Default `https://cloud-api.nobus.io` |
| `projectId` | `NOBUS_PROJECT_ID` | Default project |
| `availabilityZone` | `NOBUS_AVAILABILITY_ZONE` | Optional default only |

## Import

Resource IDs are scoped:

```text
{availabilityZone}/{projectId}/{resourceId}
```

Example:

```bash
pulumi import nobus:index:Instance demo ng1/PROJECT_UUID/SERVER_UUID
```

Volume attachments use `az/projectId/volumeId:instanceId`.

## Notable outputs

- **Instance**: `privateIpv4`, `publicIpv4`, `ipv4Addresses`, `ipv6Addresses`, `networks`, `status`
- **Volume**: `status`, `attachedTo`, `device`, `isMultiattach`
- **VolumeSnapshot** / **VolumeBackup**: `status`, `size`, `createdAt`
- **Network**: `subnetId`, `routerId`, `status`, `mtu`, `subnets`

## Build and install

```bash
make build
make install          # copies plugin into ~/.pulumi/plugins/resource-nobus-v0.1.0
# or for one-off runs:
export PULUMI_DEBUG_PROVIDERS=nobus=$PWD/bin/pulumi-resource-nobus
```

Regenerate schema + Go SDK:

```bash
make sdk
```

## Example (Go)

```bash
export NOBUS_API_KEY=...
export NOBUS_PROJECT_ID=...
export NOBUS_AVAILABILITY_ZONE=ng1
export PULUMI_DEBUG_PROVIDERS=nobus=$PWD/bin/pulumi-resource-nobus

cd examples/go/basic
pulumi stack init dev
pulumi config set nobus:projectId "$NOBUS_PROJECT_ID"
pulumi config set --secret nobus:apiKey "$NOBUS_API_KEY"
pulumi up
```

## Layout

```
provider/          # plugin source (pulumi-go-provider)
sdk/go/nobus/      # generated Go SDK
examples/go/basic/ # sample program
schema.json        # provider schema
```

## Auth

Use an API key from the Nobus dashboard or mint one via the login/OTP flow (see `get-nobus-api-key.sh` in nobus-csi). Dashboard JWT cookies are not used.

## Repository location

This tree currently lives at `nobus-csi/pulumi-nobus` so it can be developed alongside the CSI driver. It is a standalone Go module (`github.com/brimble/pulumi-nobus`) and can be moved to its own GitHub repo without code changes. Local `third_party/` copies are only for offline/sandbox builds and are gitignored — run `go mod tidy` with network access on a normal machine.
