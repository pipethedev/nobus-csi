job "nobus-csi-controller" {
  datacenters = ["dc1"]
  type        = "service"

  group "controller" {
    count = 2

    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    task "plugin" {
      driver = "docker"

      config {
        image = "ghcr.io/pipethedev/nobus-csi:latest-controller"
        args  = ["-mode=controller"]
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
NOBUS_TOKEN={{ with nomadVar "nobus/csi" }}{{ .token }}{{ end }}
EOH
      }

      csi_plugin {
        id                     = "csi.nobus.io"
        type                   = "controller"
        mount_dir              = "/csi"
        stage_publish_base_dir = "/local/csi"
        health_timeout         = "30s"
      }
    }
  }
}
