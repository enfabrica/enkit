package main

import (
	"context"
	"fmt"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
	"google.golang.org/grpc"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

func run(timeout time.Duration, cmd string, args ...string) {
	channel := make(chan bool)
	defer close(channel)
	go func(status chan bool, cmd string, args ...string) {
		job := exec.Command(cmd, args...)
		err := job.Run()
		if err != nil {
			status <- true
		} else {
			status <- false
		}
	}(channel, cmd, args...)
	go func() {
		time.Sleep(timeout * time.Second)
		log.Fatalf("Job \"%s %s\" timeout exceeded after %d seconds \n", cmd, strings.Join(args, " "), timeout)
	}()
	status := <-channel
	if status {
		log.Fatalf("Job failed to complete: %s %s \n", cmd, strings.Join(args, " "))
	} else {
		log.Printf("Job completed successfully: %s %s \n", cmd, strings.Join(args, " "))
	}
}

func polling(client rpc_license.LicenseClient, username string, quantity int32, vendor string, feature string,
	cmd string, args ...string) {
	timeout := 1800 * time.Second
	waiting := 0 * time.Second
	interval := 5 * time.Second
	stream, err := client.Polling(context.Background())
	if err != nil {
		log.Fatalf("Failed to get stream object: %s \n", err)
	}
	hash := ""
	acquired := false
	for waiting < timeout {
		request := rpc_license.PollingRequest{Vendor: vendor, Feature: feature, Quantity: quantity, User: username, Hash: hash}
		err := stream.Send(&request)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error sending message: %s \n", err)
		}
		recv, err := stream.Recv()
		hash = recv.Hash
		if recv.Acquired || err == io.EOF {
			acquired = recv.Acquired
			break
		}
		if err != nil {
			log.Fatalf("Error receiving response: %s \n", err)
		}
		// no need to continuously log after the initial request
		if waiting == 0 {
			log.Printf("Client %s waiting for %d %s feature %s license \n", hash, quantity, vendor, feature)
		}
		time.Sleep(interval)
		waiting += interval
	}
	if waiting >= timeout {
		log.Fatalf("Failed to acquire %d %s feature %s license after %d seconds \n", quantity, vendor, feature, waiting)
	}
	if acquired {
		log.Printf("Successfully acquired %d %s feature %s license from server \n", quantity, vendor, feature)
		// 12 hours in seconds
		run(60*60*12, cmd, args...)
	}
}

func main() {
	var quantity int32 = 1
	host, port := os.Args[1], os.Args[2]
	vendor, feature := os.Args[3], os.Args[4]
	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatalf("Connection failed: %s \n", err)
	}
	client := rpc_license.NewLicenseClient(conn)
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s \n", err)
	}
	polling(client, user.Username, quantity, vendor, feature, os.Args[5], os.Args[6:]...)
}
