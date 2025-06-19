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
	topology_name = flag.String("topology_name", "", "Specify a specific, known topology you wish to allocate")
	timeout = flag.Duration("timeout", 7200*time.Second, "Max time waiting in queue")
	purpose = flag.String("purpose", "", "What this reservation is for")
)

func main() {
	// This argument handling is a bit unorthodox, but must be compatible with the
	// commandline issued by bazel rules.
	flag.Parse()
	args := flag.Args()
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: $0 [flags] host port cmd [flags and args...]")
		flag.PrintDefaults()
		os.Exit(1)
	}
	host, port := args[0], args[1]
	cmd, args := args[2], args[3:]
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s\n", err)
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s\n", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func(cancelFunc func()) {
		sig := <-sigs
		log.Printf("Allocation Manager client caught signal %v; killing job...", sig)
		cancel()
	}(cancel)

	c := client.New(apb.NewAllocationManagerClient(conn), *topology_name, user.Username, *purpose)
	err = c.Guard(ctx, cmd, args...)
	if err != nil {
		log.Fatal(err)
	}
}
