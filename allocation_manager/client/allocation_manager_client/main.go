package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"time"

	"github.com/enfabrica/enkit/allocation_manager/client"
	apb "github.com/enfabrica/enkit/allocation_manager/proto"

	//	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	timeout = flag.Duration("timeout", 7200*time.Second, "Max time waiting in queue")
	purpose = flag.String("purpose", "", "What this reservation is for (TODO: test target?)")
)

func main() {
	// This argument handling is a bit unorthodox, but must be compatible with the
	// commandline issued by bazel rules.
	flag.Parse()
	args := flag.Args()
	if len(args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: $0 [flags] host port config_name config_filename cmd args")
		flag.PrintDefaults()
		os.Exit(1)
	}
	host, port := args[0], args[1]
	name, configName := args[2], args[3]
	cmd, args := args[4], args[5:]

	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s\n", err)
	}

	fh, err := os.Open(configName)
	if err != nil {
		log.Fatalf("Failed to open %s: %s\n", configName, err)
	}
	var configBytes []byte
	_, err = fh.Read(configBytes)
	if err != nil {
		log.Fatalf("Failed to read %s: %s\n", configName, err)
	}
	config := string(configBytes)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s \n", err)
	}
	defer conn.Close()

	/*
		id, err := uuid.NewRandom()
		if err != nil {
			log.Fatalf("failed to generate job ID: %w", err)
		}
	*/

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func(cancelFunc func()) {
		sig := <-sigs
		log.Printf("Allocation Manager client caught signal %v; killing job...", sig)
		cancel()
	}(cancel)

	// func New(client apb.AllocationClient, name, config, username, purpose string) *AllocationClient {
	c := client.New(apb.NewAllocationManagerClient(conn), name, config, user.Username, *purpose) //, id.String())
	err = c.Guard(ctx, cmd, args...)
	if err != nil {
		log.Fatal(err)
	}
}
