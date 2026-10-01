package main

import (
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/brimble/pulumi-nobus/provider/pkg/provider"
)

func main() {
	err := p.RunProvider("nobus", provider.Version, provider.Provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
