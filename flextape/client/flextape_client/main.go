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

	"github.com/enfabrica/enkit/flextape/client"
	fpb "github.com/enfabrica/enkit/flextape/proto"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	timeout = flag.Duration("timeout", 7200*time.Second, "Max time waiting in license queue")
)

func main() {
	// This argument handling is a bit unorthodox, but must be compatible with the
	// commandline issued by bazel rules.
	host, port := os.Args[1], os.Args[2]
	flag.Parse()
	vendor, feature, cmd, args := os.Args[3], os.Args[4], os.Args[5], os.Args[6:]

	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s \n", err)
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s \n", err)
	}
	defer conn.Close()

	id, err := uuid.NewRandom()
	if err != nil {
		log.Fatalf("failed to generate job ID: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func(cancelFunc func()) {
		sig := <-sigs
		log.Printf("Flextape client caught signal %v; killing job...", sig)
		cancel()
	}(cancel)

	c := client.New(fpb.NewFlextapeClient(conn), user.Username, vendor, feature, id.String())
	err = c.Guard(ctx, cmd, args...)
	if err != nil {
		log.Fatal(err)
	}
}
