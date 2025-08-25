package main

import (
	"flag"
	"fmt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"os"

	"github.com/enfabrica/enkit/experimental/remote_asset_service/asset_service"
)

func run(config asset_service.Config) error {
	proxyCache, err := asset_service.NewCacheProxy(config)
	if err != nil {
		return err
	}

	urlFilter := asset_service.NewUrlFilter(config)
	metrics := asset_service.NewMetrics()
	assetDownloader := asset_service.NewAssetDownloader(config, proxyCache, urlFilter, metrics)

	servers := new(errgroup.Group)
	servers.Go(func() error {
		grpcAddress := config.GrpcAddress()

		var opts []grpc.ServerOption

		grpcServer := grpc.NewServer(opts...)

		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			return err
		}

		log.Println("Starting gRPC server on address", grpcAddress)

		asset_service.RegisterAssetServer(config, grpcServer, proxyCache, assetDownloader)

		h := health.NewServer()
		grpc_health_v1.RegisterHealthServer(grpcServer, h)
		h.SetServingStatus("/grpc.health.v1.Health/Check", grpc_health_v1.HealthCheckResponse_SERVING)

		return grpcServer.Serve(listener)
	})

	return servers.Wait()
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: asset_service [config.yaml]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Config file is missing.")
	}

	config, err := asset_service.NewConfigFromPath(args[0])
	if err != nil {
		log.Fatalf("Parse config error: %v", err)
	}

	err = run(config)
	if err != nil {
		log.Fatalf("Run server error: %v", err)
	}
}
