job "nobus-csi-node" {
  datacenters = ["dc1"]
  type        = "system"

  group "node" {
    disconnect {
      lost_after = "24h"
      replace    = false
      reconcile  = "keep_original"
    }

    task "plugin" {
      driver = "docker"

      config {
        image      = "ghcr.io/pipethedev/nobus-csi:latest-node"
        force_pull = true
        privileged = true
        volumes = [
          "/run/cloud-init:/run/cloud-init:ro",
          "/var/lib/cloud:/var/lib/cloud:ro",
        ]
        args = [
          "-endpoint=unix:///csi/csi.sock",
          "-mode=node",
        ]
      }

      env {
        NOBUS_API_URL           = "https://cloud-api.nobus.io"
        NOBUS_PROJECT_ID        = "replace-me"
        NOBUS_AVAILABILITY_ZONE = "replace-me"
      }

      template {
        destination = "secrets/nobus.env"
        env         = true
        data        = <<EOH
NOBUS_API_KEY={{ with nomadVar "nobus/csi" }}{{ .api_key }}{{ end }}
EOH
      }

      csi_plugin {
        id                     = "csi.nobus.io"
        type                   = "node"
        mount_dir              = "/csi"
        stage_publish_base_dir = "/local/csi"
        health_timeout         = "30s"
      }
    }
  }
}
