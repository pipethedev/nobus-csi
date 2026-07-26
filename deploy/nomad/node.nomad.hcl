job "nobus-csi-node" {
  datacenters = ["dc1"]
  type        = "system"

  group "node" {
    task "plugin" {
      driver = "docker"

      config {
        image      = "ghcr.io/pipethedev/nobus-csi:latest-node"
        privileged = true
        args       = ["-mode=node"]

        volumes = [
          "/dev:/dev",
          "/run/udev:/run/udev",
          "/sys:/sys"
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
NOBUS_EMAIL={{ with nomadVar "nobus/csi" }}{{ .email }}{{ end }}
NOBUS_PASSWORD={{ with nomadVar "nobus/csi" }}{{ .password }}{{ end }}
NOBUS_TOKEN={{ with nomadVar "nobus/csi" }}{{ .token }}{{ end }}
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
