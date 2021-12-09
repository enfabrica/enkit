package main

import (
	"context"
	"fmt"
	"net/http"

	hpb "github.com/enfabrica/enkit/bestie/proto"
	"github.com/enfabrica/enkit/lib/server"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

type GreeterService struct {
}

func (s *GreeterService) Greet(ctx context.Context, req *hpb.GreetRequest) (*hpb.GreetResponse, error) {
	return &hpb.GreetResponse{
		Greeting: fmt.Sprintf("Hello, %s!", req.GetName()),
	}, nil
}

func main() {
	grpcs := grpc.NewServer()
	s := &GreeterService{}
	hpb.RegisterGreeterServer(grpcs, s)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server.CloudRun(mux, grpcs)
}
