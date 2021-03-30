package main

import (
	"os"
	"time"
	"log"
	"context"
	"google.golang.org/grpc"
	"strconv"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
)

func main() {
	timeout := 1800 * time.Second
	interval := 60 * time.Second
	vendor, feature:= os.Args[1], os.Args[2]
	quantity, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Failed to convert %s to an int \n", os.Args[3])
	}
	conn, err := grpc.Dial(":8080", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s \n", err)
	}
	client := rpc_license.NewLicenseClient(conn)
	response, err := client.Acquire(context.Background(), &rpc_license.AcquireRequest{Vendor: vendor, Feature: feature, Quantity: int32(quantity)})
	if err != nil {
		log.Fatalf("Error calling client.Acquire: %s \n", err)
	}
	defer conn.Close()
	waiting := 0 * time.Second
	for response.Waiting && waiting < timeout {
		time.Sleep(interval)
		waiting += interval
		response, err = client.Acquire(context.Background(), &rpc_license.AcquireRequest{Vendor: vendor, Feature: feature, Quantity: int32(quantity)})
		if err != nil {
			log.Fatalf("Error calling client.Acquire: %s \n", err)
		}
	}
	if waiting >= timeout {
		log.Fatalf("Failed to acquire %d %s feature %s license after %d seconds \n", quantity, vendor, feature, waiting)
	} else if response.Available {
		// Starting a keepalive hearbeat session with the server here would be a blocking call
		log.Printf("Successfully acquired %d %s feature %s license from server \n", quantity, vendor, feature)
	} else if response.Missing {
		log.Fatalf("%s feature %s was not found on the license server \n", vendor, feature)
	} else {
		log.Fatalf("Invalid response from server: Available=%t, Waiting=%t, Missing=%t \n", response.Available, response.Waiting, response.Missing)
	}
}
