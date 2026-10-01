package main

import (
	"os"

	"github.com/brimble/pulumi-nobus/sdk/go/nobus"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		projectID := os.Getenv("NOBUS_PROJECT_ID")
		az := os.Getenv("NOBUS_AVAILABILITY_ZONE")
		if az == "" {
			az = nobus.AvailabilityZoneNG1
		}

		flavorName := os.Getenv("NOBUS_FLAVOR")
		if flavorName == "" {
			flavorName = nobus.FlavorS1Small
		}
		flavor, err := nobus.GetFlavor(ctx, &nobus.GetFlavorArgs{
			NameOrId:         flavorName,
			AvailabilityZone: az,
			ProjectId:        strPtr(projectID),
		})
		if err != nil {
			return err
		}

		imageName := os.Getenv("NOBUS_IMAGE")
		if imageName == "" {
			imageName = "Ubuntu"
		}
		image, err := nobus.GetImage(ctx, &nobus.GetImageArgs{
			NameOrId:         imageName,
			AvailabilityZone: az,
			ProjectId:        strPtr(projectID),
		})
		if err != nil {
			return err
		}

		network, err := nobus.NewNetwork(ctx, "demo-net", &nobus.NetworkArgs{
			NetworkName:      pulumi.String("pulumi-demo-net"),
			SubnetCidr:       pulumi.String("10.42.0.0/24"),
			RouterName:       pulumi.String("pulumi-demo-router"),
			AvailabilityZone: pulumi.String(az),
		})
		if err != nil {
			return err
		}

		kp, err := nobus.NewKeyPair(ctx, "demo-key", &nobus.KeyPairArgs{
			Name:             pulumi.String("pulumi-demo-key"),
			AvailabilityZone: pulumi.String(az),
		})
		if err != nil {
			return err
		}

		instance, err := nobus.NewInstance(ctx, "demo-vm", &nobus.InstanceArgs{
			Name:             pulumi.String("pulumi-demo-vm"),
			Image:            pulumi.String(image.ImageId),
			Flavor:           pulumi.String(flavor.FlavorId),
			KeyName:          kp.Name,
			NetworkId:        network.ID(),
			AvailabilityZone: pulumi.String(az),
		})
		if err != nil {
			return err
		}

		volume, err := nobus.NewVolume(ctx, "demo-vol", &nobus.VolumeArgs{
			Name:             pulumi.String("pulumi-demo-vol"),
			Size:             pulumi.Int(10),
			AvailabilityZone: pulumi.String(az),
		})
		if err != nil {
			return err
		}

		_, err = nobus.NewVolumeAttachment(ctx, "demo-attach", &nobus.VolumeAttachmentArgs{
			VolumeId:         volume.ID(),
			InstanceId:       instance.ID(),
			AvailabilityZone: pulumi.String(az),
		})
		if err != nil {
			return err
		}

		ctx.Export("instanceId", instance.ID())
		ctx.Export("privateIpv4", instance.PrivateIpv4)
		ctx.Export("publicIpv4", instance.PublicIpv4)
		ctx.Export("networkId", network.ID())
		ctx.Export("volumeId", volume.ID())
		ctx.Export("keyName", kp.Name)
		return nil
	})
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
