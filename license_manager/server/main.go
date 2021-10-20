package main

import (
	"github.com/enfabrica/enkit/lib/server"
	lmpb "github.com/enfabrica/enkit/license_manager/proto"
	"github.com/enfabrica/enkit/license_manager/service"

	"google.golang.org/grpc"
)

func main() {
	grpcs := grpc.NewServer()
	s := service.New()
	lmpb.RegisterLicenseManagerServer(grpcs, s)

	server.Run(nil, grpcs)
}
