package main

import (
	common "github.com/enfabrica/enkit/manager/common"
	rpc "github.com/enfabrica/enkit/manager/rpc"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	listen, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var server = grpc.NewServer()
	rpc.RegisterLicenseServer(server, &common.Server{})
	err = server.Serve(listen)
	if err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
