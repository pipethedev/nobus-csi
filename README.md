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

## Current Provider Gap

The Nobus OpenAPI schema documents block storage APIs, but it does not expose a verified instance metadata endpoint. Node identity must eventually come from Nobus provider metadata; until that endpoint is confirmed, real node-mode startup should be treated as incomplete.
