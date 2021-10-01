package main

import (
	"flag"
	"net"
	"strconv"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"
	"github.com/enfabrica/enkit/license_manager/service"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port = flag.Int("port", 8080, "Port for gRPC services")
)

func main (){
	flag.Parse()

	addr := net.JoinHostPort("", strconv.FormatInt(int64(*port), 10))
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		glog.Fatalf("Failed to listen on %q: %v", addr, err)
	}
	server := grpc.NewServer()
	lmpb.RegisterLicenseManagerServer(server, &service.Service{})
	reflection.Register(server)

	glog.Infof("Listening for gRPC requests on %s...", addr)
	err = server.Serve(listen)
	if err != nil {
		glog.Fatalf("Failed to serve: %v", err)
	}
}