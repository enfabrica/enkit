package main

import (
	"fmt"
	common "github.com/enfabrica/enkit/manager/common"
	rpc "github.com/enfabrica/enkit/manager/rpc"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

func main() {
	port := fmt.Sprintf(":%s", os.Args[1])
	listen, err := net.Listen("tcp", port)
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
