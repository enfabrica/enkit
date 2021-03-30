package main

import (
	"log"
	"net"
	"google.golang.org/grpc"
	rpc "github.com/enfabrica/enkit/manager/rpc"
	common "github.com/enfabrica/enkit/manager/common"
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
