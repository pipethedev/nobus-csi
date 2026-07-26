# Nobus CSI Driver

`nobus-csi` is a Container Storage Interface driver for Nobus Cloud block storage. It is built as one binary that can run as a Nomad controller, Nomad node plugin, Nomad monolith plugin, or Kubernetes CSI driver.

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

## Runtime Config

| Name | Required | Default | Description |
|---|---:|---|---|
| `CSI_ENDPOINT` | no | `unix:///csi/csi.sock` | CSI unix socket endpoint. `tcp://` is rejected. |
| `NOBUS_API_URL` | yes for real provider | none | Nobus API base URL, for example `https://cloud-api.nobus.io`. |
| `NOBUS_TOKEN` | yes for real provider | none | Nobus bearer token. Never put this directly in jobspecs or manifests. |
| `NOBUS_PROJECT_ID` | yes | none | Nobus project ID used for volume APIs. |
| `NOBUS_AVAILABILITY_ZONE` | yes | none | Nobus availability zone used for volume APIs. |
| `NOBUS_VOLUME_TYPE` | no | none | Default Nobus volume type. |
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

Use `stage_publish_base_dir` with a non-default path in tests. For single-writer volumes, avoid rolling update settings that force a new allocation to claim the same volume before the old allocation releases it.

## Kubernetes

The Kubernetes manifests are under `deploy/kubernetes`. They use CSI sidecars, but the driver itself does not import Kubernetes clients or depend on Kubernetes-only volume context.

```sh
kubectl apply -k deploy/kubernetes
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

The OpenAPI does not document conditional volume or snapshot creation by name. After create, the driver re-lists the requested name in the target AZ, returns the lowest compatible provider ID, and deletes only the duplicate object created by the current request if it loses that canonical selection.

CSI snapshot IDs returned by this driver are `availability-zone/provider-snapshot-id` so `DeleteSnapshot` can call the Nobus AZ-scoped delete endpoint later. Registered pre-existing snapshots may still use the raw provider snapshot ID, in which case delete falls back to `NOBUS_AVAILABILITY_ZONE`.

## Node Identity

The Nobus OpenAPI schema documents block storage APIs but not an instance metadata API. The node plugin reads provider metadata from cloud-init ConfigDrive data at `/run/cloud-init/instance-data.json` or `/var/lib/cloud/instance/instance-data.json`. If neither file has an instance ID, `NodeGetInfo` fails instead of serving a bogus node ID.
