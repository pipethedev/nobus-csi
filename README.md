# Nobus CSI Driver

`nobus-csi` is a Container Storage Interface driver for Nobus Cloud block storage. It is built as one binary that can run as a Nomad controller, Nomad node plugin, Nomad monolith plugin, or Kubernetes CSI driver.

## Nobus API Reference

The provider integration is based on the [Nobus](https://nobus.io) cloud platform, the [Nobus API docs](https://cloud-api.nobus.io/api/v3/docs), and the [Nobus OpenAPI spec](https://cloud-api.nobus.io/api/v3/openapi.json).

Authentication uses a pre-issued API key sent as `Authorization: Bearer <api-key>` on block storage API calls. The driver does not accept Nobus login credentials or create and refresh API keys.

## Build

```sh
go build ./...
go test -race ./...
golangci-lint run
```

Build the Linux binary for containers:

```sh
GOOS=linux GOARCH=amd64 go build -o nobus-csi ./cmd/nobus-csi
```

## CI/CD

GitHub Actions runs build, vet, race tests, and lint on pull requests and pushes. Pushes to `main` publish these images to GitHub Container Registry using the built-in `GITHUB_TOKEN`:

```text
ghcr.io/pipethedev/nobus-csi:latest-controller
ghcr.io/pipethedev/nobus-csi:latest-node
```

Both images are published for `linux/amd64` and `linux/arm64`.

To also deploy the Nomad CSI jobs on every `main` push, configure these repository settings:

| Type | Name | Description |
|---|---|---|
| Secret | `NOMAD_ADDR` | Nomad API address reachable from GitHub Actions. |
| Secret | `NOMAD_TOKEN` | Nomad token allowed to run the CSI jobs. |
| Variable | `NOMAD_DEPLOY` | Set to `true` to enable deploys. |
| Variable | `NOBUS_PROJECT_ID` | Nobus project ID rendered into the jobspecs. |
| Variable | `NOBUS_AVAILABILITY_ZONE` | Nobus AZ rendered into the jobspecs. |

The Nobus API key lives in Nomad itself via `nomad var put nobus/csi api_key=...`; it is not stored in GitHub Actions.

## Runtime Config

| Name | Required | Default | Description |
|---|---:|---|---|
| `CSI_ENDPOINT` | no | `unix:///csi/csi.sock` | CSI unix socket endpoint. `tcp://` is rejected. |
| `NOBUS_API_URL` | yes for real provider | none | Nobus API base URL, for example `https://cloud-api.nobus.io`. |
| `NOBUS_API_KEY` | yes for real provider | none | Pre-issued Nobus API key sent as a bearer credential. |
| `NOBUS_PROJECT_ID` | yes | none | Nobus project ID used for volume APIs. |
| `NOBUS_AVAILABILITY_ZONE` | yes | none | Nobus availability zone used for volume APIs. |
| `NOBUS_VOLUME_TYPE` | no | none | Default Nobus volume type. |
| `NOBUS_ALLOW_FAKE` | no | `false` | Enables the in-memory fake provider and allows node/all mode on non-Linux hosts for local tests only. Without this flag, node/all mode refuses to start on non-Linux so a real provider cannot be paired with a fake mounter. Do not set this in Nomad or Kubernetes. |
| `NOBUS_DRIVER_NAME` | no | `csi.nobus.io` | CSI driver name. |
| `NOBUS_DRIVER_VERSION` | no | `0.1.0` | Version shown by Nomad and Kubernetes health tooling. |

## Nomad

Nomad is the primary target. Use the Docker task driver for node plugins because the driver needs host block-device access.

```sh
nomad job run deploy/nomad/controller.nomad.hcl
nomad job run deploy/nomad/node.nomad.hcl
nomad plugin status csi.nobus.io
nomad volume create deploy/nomad/example-volume.hcl
nomad volume register deploy/nomad/register-volume.hcl
```

Store the API key with `nomad var put nobus/csi api_key="$NOBUS_API_KEY"`. Use `stage_publish_base_dir` with a non-default path in tests. For single-writer volumes, avoid rolling update settings that force a new allocation to claim the same volume before the old allocation releases it.

### Restart Safety

CSI plugins are not in the data path after a volume is mounted. A Nomad client-agent restart must not detach a still-running workload. Keep the Nomad client `data_dir` on persistent host storage so volume claims and mount metadata survive the restart.

The controller and node plugin jobs already keep their allocations during a client disconnect. Every stateful workload group that uses this driver should do the same:

```hcl
disconnect {
  lost_after = "24h"
  replace    = false
  reconcile  = "keep_original"
}
```

Drain a node with `nomad node drain` before an intentional shutdown so plugins unmount and unpublish after the workload stops. If a node is permanently lost, fence it and migrate the volume by hand. Nomad will not automatically move a single-writer volume that may still be mounted.

## Kubernetes

The Kubernetes manifests are under `deploy/kubernetes`. They use CSI sidecars, but the driver itself does not import Kubernetes clients or depend on Kubernetes-only volume context.

```sh
kubectl apply -k deploy/kubernetes
```

Create the `nobus-csi` Secret with `NOBUS_API_KEY`.

The Helm chart is under `deploy/helm/nobus-csi`:

```sh
kubectl -n kube-system create secret generic nobus-csi \
  --from-literal=NOBUS_API_URL=https://cloud-api.nobus.io \
  --from-literal=NOBUS_API_KEY="$NOBUS_API_KEY" \
  --from-literal=NOBUS_PROJECT_ID="$NOBUS_PROJECT_ID" \
  --from-literal=NOBUS_AVAILABILITY_ZONE="$NOBUS_AVAILABILITY_ZONE"

helm install nobus-csi deploy/helm/nobus-csi \
  --namespace kube-system \
  --set nobus.existingSecret=nobus-csi \
  --set nobus.projectId="$NOBUS_PROJECT_ID" \
  --set nobus.availabilityZone="$NOBUS_AVAILABILITY_ZONE"
```

## Supported Parameters

| Key | Type | Default | Mutable | Description |
|---|---|---|---:|---|
| `project_id` | string | `NOBUS_PROJECT_ID` | no | Nobus project used for the volume. |
| `availability_zone` | string | `NOBUS_AVAILABILITY_ZONE` | no | Nobus availability zone. |
| `volume_type` | string | `NOBUS_VOLUME_TYPE` | no | Nobus volume type. |
| `region` | string | none | no | Reserved for topology context. |

Unknown parameters are rejected with `INVALID_ARGUMENT`.

## Provider Limits

The Nobus OpenAPI currently exposes AZ-scoped list endpoints without server-side page-token or page-size parameters. `ListVolumes` and `ListSnapshots` therefore list the configured `NOBUS_AVAILABILITY_ZONE` and apply CSI pagination locally.

The documented v3 get-volume endpoint currently fails for valid volume IDs. `GetVolumeByID` therefore uses the AZ-scoped list endpoint and selects the exact provider ID locally.

Volume attach and detach use the public v3 `/api/v3/volume/attach` and `/api/v3/volume/detach` endpoints with API-key authentication.

The OpenAPI includes `multiattach` on create-volume, but the live API rejects that field. The driver does not send it during create and still rejects any out-of-band volume that Nobus reports as multiattach.

The OpenAPI does not document conditional volume or snapshot creation by name. After create, the driver re-lists the requested name in the target AZ, waits for the lowest compatible provider ID to stay stable, returns that canonical ID, and removes visible compatible duplicates when they are safe to delete. This is best-effort reconciliation, not a replacement for provider-enforced unique names.

CSI snapshot IDs returned by this driver are `availability-zone/provider-snapshot-id` so `DeleteSnapshot` can call the Nobus AZ-scoped delete endpoint later. Registered pre-existing snapshots may still use the raw provider snapshot ID, in which case delete falls back to `NOBUS_AVAILABILITY_ZONE`.

## Node Identity

Nobus documents instance management APIs, but the current OpenAPI does not expose a “current node” metadata endpoint the CSI node plugin can query. The node plugin therefore reads instance identity from cloud-init ConfigDrive data at `/run/cloud-init/instance-data.json` or `/var/lib/cloud/instance/instance-data.json`. If neither file has an instance ID, `NodeGetInfo` fails instead of serving a bogus node ID.
