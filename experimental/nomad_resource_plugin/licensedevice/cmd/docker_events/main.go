package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/docker"
)

func exitIf(err error) {
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	c, err := docker.NewClient(ctx)
	exitIf(err)

	eventsChan := c.Chan(ctx)

	fmt.Println("Listening for events...")

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-eventsChan:
			inUse, err := c.GetCurrent(ctx)
			exitIf(err)
			fmt.Printf("Licenses in use: ")
			for _, l := range inUse {
				fmt.Printf("%s ", l.ID)
			}
			fmt.Printf("\n")
		}
	}

	<-ctx.Done()
}
