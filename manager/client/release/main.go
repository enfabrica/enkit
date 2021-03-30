package main

import (
	"os"
	"strconv"
	"log"
	"context"
	"google.golang.org/grpc"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
)

func main() {
	vendor, feature := os.Args[1], os.Args[2]
	quantity, err := strconv.Atoi(os.Args[3])
    if err != nil {
        log.Fatalf("Failed to convert %s to an int \n", os.Args[3])
    }
	conn, err := grpc.Dial(":8080", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Connection failed: %s", err)
	}
	client := rpc_license.NewLicenseClient(conn)
	response, err := client.Release(context.Background(), &rpc_license.ReleaseRequest{Vendor: vendor, Feature: feature, Quantity: int32(quantity)})
	if err != nil {
		log.Fatalf("Error calling client.Release: %s", err)
	}
	defer conn.Close()
	if response.Success {
		log.Printf("Successfully released %d %s feature %s \n", quantity, vendor, feature)
	} else {
		log.Fatalf("Failed to release %d %s feature %s \n", quantity, vendor, feature)
	}
}
