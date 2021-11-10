package main

import (
	"github.com/enfabrica/enkit/lib/server"
	fpb "github.com/enfabrica/enkit/flextape/proto"
	"github.com/enfabrica/enkit/flextape/service"

	"google.golang.org/grpc"
)

func main() {
	grpcs := grpc.NewServer()
	s := service.New()
	fpb.RegisterFlextapeServer(grpcs, s)

	server.Run(nil, grpcs)
}
