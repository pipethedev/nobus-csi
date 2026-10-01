#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
append_once() {
  local file="$1"
  local line="$2"
  grep -qxF "$line" "$file" 2>/dev/null || printf '\n%s\n' "$line" >> "$file"
}
append_once "$root/provider/go.mod" 'replace github.com/pulumi/pulumi-go-provider => ../third_party/pulumi-go-provider'
append_once "$root/provider/go.mod" 'replace github.com/pulumi/pulumi/sdk/v3 => ../third_party/pulumi-sdk'
append_once "$root/provider/go.mod" 'replace github.com/pulumi/pulumi/pkg/v3 => ../third_party/pulumi-pkg'
append_once "$root/provider/go.mod" 'replace github.com/pulumi/esc => ../third_party/esc'
append_once "$root/sdk/go/go.mod" 'replace github.com/pulumi/pulumi/sdk/v3 => ../../third_party/pulumi-sdk'
append_once "$root/sdk/go/go.mod" 'replace github.com/pulumi/esc => ../../third_party/esc'
append_once "$root/examples/go/basic/go.mod" 'replace github.com/pulumi/pulumi/sdk/v3 => ../../../third_party/pulumi-sdk'
append_once "$root/examples/go/basic/go.mod" 'replace github.com/pulumi/esc => ../../../third_party/esc'
echo "Local third_party replaces applied."
