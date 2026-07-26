# Nobus CSI Driver

`nobus-csi` is a Container Storage Interface driver for Nobus Cloud block storage. It is built as one binary that can run as a Nomad controller, Nomad node plugin, Nomad monolith plugin, or Kubernetes CSI driver.

## Nobus API Reference

The provider integration is based on the Nobus API docs:

```text
https://nobus.io
https://cloud-api.nobus.io/api/v2/docs
https://cloud-api.nobus.io/api/v2/openapi.json
```

Authentication uses `POST /api/v2/auth/login` with email and password, then sends the returned token as `Authorization: Bearer <token>` on block storage API calls.

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

The Nobus login email/password still live in Nomad itself via `nomad var put nobus/csi`; they are not stored in GitHub Actions.

## Runtime Config

| Name | Required | Default | Description |
|---|---:|---|---|
| `CSI_ENDPOINT` | no | `unix:///csi/csi.sock` | CSI unix socket endpoint. `tcp://` is rejected. |
| `NOBUS_API_URL` | yes for real provider | none | Nobus API base URL, for example `https://cloud-api.nobus.io`. |
| `NOBUS_TOKEN` | no | none | Nobus bearer token from `/api/v2/auth/login`, not an API key. When set, it is used directly and login is skipped until a 401 response triggers refresh if email/password are also set. |
| `NOBUS_EMAIL` | yes if `NOBUS_TOKEN` is empty | none | Nobus login email. |
| `NOBUS_PASSWORD` | yes if `NOBUS_TOKEN` is empty | none | Nobus login password. |
| `NOBUS_PROJECT_ID` | yes | none | Nobus project ID used for volume APIs. |
| `NOBUS_AVAILABILITY_ZONE` | yes | none | Nobus availability zone used for volume APIs. |
| `NOBUS_VOLUME_TYPE` | no | none | Default Nobus volume type. |
| `NOBUS_ALLOW_FAKE` | no | `false` | Enables the in-memory fake provider for local tests only. Do not set this in Nomad or Kubernetes. |
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

Store `email` and `password` in `nomad var put nobus/csi`. You can also store `token` there if you prefer to inject a pre-issued bearer token. Use `stage_publish_base_dir` with a non-default path in tests. For single-writer volumes, avoid rolling update settings that force a new allocation to claim the same volume before the old allocation releases it.

## Kubernetes

The Kubernetes manifests are under `deploy/kubernetes`. They use CSI sidecars, but the driver itself does not import Kubernetes clients or depend on Kubernetes-only volume context.

```sh
kubectl apply -k deploy/kubernetes
```

Create the `nobus-csi` Secret with `NOBUS_EMAIL` and `NOBUS_PASSWORD`, or with `NOBUS_TOKEN` if you want to inject a pre-issued bearer token.

The Helm chart is under `deploy/helm/nobus-csi`:

```sh
kubectl -n kube-system create secret generic nobus-csi \
  --from-literal=NOBUS_API_URL=https://cloud-api.nobus.io \
  --from-literal=NOBUS_PROJECT_ID="$NOBUS_PROJECT_ID" \
  --from-literal=NOBUS_AVAILABILITY_ZONE="$NOBUS_AVAILABILITY_ZONE" \
  --from-literal=NOBUS_EMAIL="$NOBUS_EMAIL" \
  --from-literal=NOBUS_PASSWORD="$NOBUS_PASSWORD"

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

The OpenAPI does not document conditional volume or snapshot creation by name. After create, the driver re-lists the requested name in the target AZ, waits for the lowest compatible provider ID to stay stable, returns that canonical ID, and removes visible compatible duplicates when they are safe to delete. This is best-effort reconciliation, not a replacement for provider-enforced unique names.

CSI snapshot IDs returned by this driver are `availability-zone/provider-snapshot-id` so `DeleteSnapshot` can call the Nobus AZ-scoped delete endpoint later. Registered pre-existing snapshots may still use the raw provider snapshot ID, in which case delete falls back to `NOBUS_AVAILABILITY_ZONE`.

## Node Identity

The Nobus OpenAPI schema documents block storage APIs but not an instance metadata API. The node plugin reads provider metadata from cloud-init ConfigDrive data at `/run/cloud-init/instance-data.json` or `/var/lib/cloud/instance/instance-data.json`. If neither file has an instance ID, `NodeGetInfo` fails instead of serving a bogus node ID.
