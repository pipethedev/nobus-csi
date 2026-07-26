type      = "csi"
id        = "example-nobus-volume"
name      = "example-nobus-volume"
plugin_id = "csi.nobus.io"

capacity_min = "10GiB"
capacity_max = "10GiB"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

mount_options {
  fs_type = "ext4"
}

parameters {
  project_id        = "replace-me"
  availability_zone = "replace-me"
}
