package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"os"

	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/enfabrica/enkit/experimental/remote_asset_service/asset_service"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: asset_service [config.yaml]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

type prog struct {
	configPath string
}

func (p *prog) run(ctx context.Context, siblingsGroup, dependenciesGroup program.Group) error {
	config, err := asset_service.NewConfigFromPath(p.configPath, dependenciesGroup)
	if err != nil {
		log.Fatalf("Parse config error: %v", err)
	}

	proxyCache, err := asset_service.NewCacheProxy(config)
	if err != nil {
		return err
	}

	urlFilter := asset_service.NewUrlFilter(config)
	metrics := asset_service.NewMetrics()
	assetDownloader := asset_service.NewAssetDownloader(config, proxyCache, urlFilter, metrics)

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
}

func permutateArgs(args []string) int {
	args = args[1:]
	optind := 0

	for i := range args {
		if args[i][0] == '-' {
			tmp := args[i]
			args[i] = args[optind]
			args[optind] = tmp
			optind++
		}
	}

	return optind + 1
}

func main() {
	_ = godotenv.Load(".env")

	flag.Usage = usage

	local := flag.Bool("local", false, "Run in non daemon mode")

	optind := permutateArgs(os.Args)
	flag.Parse()
	args := os.Args[optind:]

	if len(args) != 1 {
		log.Fatalf("Single config file argument required.")
		return
	}

	p := &prog{configPath: args[0]}

	if *local {
		err := program.RunLocal(context.Background(), p.run)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		program.RunMain(p.run)
	}
}
