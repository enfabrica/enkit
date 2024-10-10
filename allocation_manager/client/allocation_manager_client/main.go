package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/enfabrica/enkit/allocation_manager/client"
	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/allocation_manager/topology"

	//	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	timeout = flag.Duration("timeout", 7200*time.Second, "Max time waiting in queue")
	purpose = flag.String("purpose", "", "What this reservation is for")
)

func main() {
	// This argument handling is a bit unorthodox, but must be compatible with the
	// commandline issued by bazel rules.
	flag.Parse()
	args := flag.Args()
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: $0 [flags] host port config_filenames cmd [flags and args...]")
		flag.PrintDefaults()
		os.Exit(1)
	}
	host, port := args[0], args[1]
	configFilenames := args[2]
	cmd, args := args[3], args[4:]
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s\n", err)
	}
	var names []string
	var configstrs []string
	for _, fn := range strings.Split(configFilenames, ",") {
		fh, err := os.Open(fn)
		defer fh.Close()
		if err != nil {
			log.Fatalf("Failed to open %s: %s\n", fn, err)
		}
		configBytes := make([]byte, 1024000) // topology limited to 1MB
		count, err := fh.Read(configBytes)
		if err != nil {
			log.Fatalf("Failed to read %s: %s\n", fn, err)
		}
		parsedTopology, err := topology.ParseYaml(configBytes[:count])
		if err != nil {
			log.Fatalf("cannot unmarshal data: %v\n", err)
		}
		names = append(names, parsedTopology.Name) // use the parsed yaml request for only the name.
		fmt.Printf("Requesting unit name %s\n", parsedTopology.Name)
		configstrs = append(configstrs, string(configBytes))
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
	fmt.Printf("names=%v, configstrs=%v\n", names, configstrs)
	c := client.New(apb.NewAllocationManagerClient(conn), names, configstrs, user.Username, *purpose)
	err = c.Guard(ctx, cmd, args...)
	if err != nil {
		log.Fatal(err)
	}
}
