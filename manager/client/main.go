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
	"time"
)

func run(status chan bool, cmd string, args ...string) error {
	job := exec.Command(cmd, args...)
	err := job.Run()
	if err != nil {
		log.Fatalf("Build failed to complete: %s \n", err)
		status <- true
		close(status)
		return err
	}
	status <- false
	close(status)
	return nil
}

func keepalive(status chan bool, client rpc_license.LicenseClient, hash string, vendor string, feature string) {
	stream, err := client.KeepAlive(context.Background())
	if err != nil {
		log.Fatalf("Failed to get stream object: %s \n", err)
	}
	for {
		stream.Send(&rpc_license.KeepAliveMessage{Hash: hash, Vendor: vendor, Feature: feature})
		recv, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if recv.Hash != hash {
			log.Fatalf("Hash from server does not match client: %s \n", recv.Hash)
		}
		if err != nil {
			log.Fatalf("Error receiving response: %s \n", err)
		}
		time.Sleep(5 * time.Second)
	}
	// failure if keepalive is disconnected before go-routine run() finishes
	status <- true
	close(status)
}

func polling(client rpc_license.LicenseClient, username string, quantity int32, vendor string, feature string) (string, error) {
	timeout := 1800 * time.Second
	waiting := 0 * time.Second
	interval := 5 * time.Second
	stream, err := client.Polling(context.Background())
	if err != nil {
		log.Fatalf("Failed to get stream object: %s \n", err)
	}
	hash := ""
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
			break
		}
		if err != nil {
			log.Fatalf("Error receiving response: %s \n", err)
		}
		log.Printf("Client %s waiting for %d %s feature %s license \n", hash, quantity, vendor, feature)
		time.Sleep(interval)
		waiting += interval
	}
	if waiting >= timeout {
		log.Fatalf("Failed to acquire %d %s feature %s license after %d seconds \n", quantity, vendor, feature, waiting)
	}
	return hash, nil
}

func main() {
	var quantity int32 = 1
	host, port := os.Args[1], os.Args[2]
	vendor, feature := os.Args[3], os.Args[4]
	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s \n", err)
	}
	client := rpc_license.NewLicenseClient(conn)
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get username: %s \n", err)
	}
	hash, err := polling(client, user.Username, quantity, vendor, feature)
	if err == nil {
		log.Printf("Successfully acquired %d %s feature %s license from server \n", quantity, vendor, feature)
		status := make(chan bool)
		go run(status, os.Args[5], os.Args[6:]...)
		go keepalive(status, client, hash, vendor, feature)
		// blocking call waiting for channel status
		isFailure := <-status
		if isFailure {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	} else {
		log.Fatalf("Invalid response from server: %s \n", err)
	}
	defer conn.Close()
}
