---
title: Nobus
meta_desc: Deploy Nobus Cloud compute, networking, and block storage with Pulumi.
layout: package
---

The Nobus Pulumi provider lets you manage [Nobus Cloud](https://cloud.nobus.io) infrastructure as code: instances, SSH key pairs, volumes, networks, security groups, and floating IPs.

## Example

```go
package main

import (
	"github.com/brimble/pulumi-nobus/sdk/go/nobus"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		kp, err := nobus.NewKeyPair(ctx, "demo", &nobus.KeyPairArgs{
			Name:             pulumi.String("demo-key"),
			AvailabilityZone: pulumi.String(nobus.AvailabilityZoneNG1),
		})
		if err != nil {
			return err
		}
		ctx.Export("keyName", kp.Name)
		return nil
	})
}
```

```typescript
import * as nobus from "@brimble/nobus";

const kp = new nobus.KeyPair("demo", {
  name: "demo-key",
  availabilityZone: "ng1",
});

export const keyName = kp.name;
```

```python
import pulumi
import brimble_nobus as nobus

kp = nobus.KeyPair("demo",
    name="demo-key",
    availability_zone="ng1")

pulumi.export("key_name", kp.name)
```

```yaml
name: nobus-demo
runtime: yaml
resources:
  demo:
    type: nobus:index:KeyPair
    properties:
      name: demo-key
      availabilityZone: ng1
outputs:
  keyName: ${demo.name}
```

See [Installation & Configuration](./installation-configuration/) for credentials, availability zones, and import IDs.
