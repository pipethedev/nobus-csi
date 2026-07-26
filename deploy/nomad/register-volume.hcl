type        = "csi"
id          = "existing-nobus-volume"
name        = "existing-nobus-volume"
external_id = "replace-with-nobus-volume-id"
plugin_id   = "csi.nobus.io"

capacity_min = "10GiB"
capacity_max = "10GiB"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

mount_options {
  fs_type = "ext4"
}

context {
  availability_zone = "replace-me"
  volume_type       = "replace-me"
}
