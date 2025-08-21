package main

import (
	"flag"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/enfabrica/enkit/experimental/remote_asset_service/asset_service"
)

var (
	port = flag.Int("port", 9092, "port")
	host = flag.String("host", "127.0.0.1", "host")

	proxyPort = flag.Int("proxy_port", 8982, "proxy_port")
	proxyHost = flag.String("proxy_host", "127.0.0.1", "proxy_host")
)

type Config struct {
	accessLogger *log.Logger
	errorLogger  *log.Logger
	grpcAddress  string
	proxyAddress string
}

func run(c *Config) error {
	proxyCache, err := asset_service.NewCacheProxy(c.proxyAddress)
	if err != nil {
		return err
	}

	assetDownloader := asset_service.NewAssetDownloader(proxyCache, c.accessLogger)

	servers := new(errgroup.Group)
	servers.Go(func() error {
		grpcAddress := net.JoinHostPort(*host, strconv.Itoa(*port))

		var opts []grpc.ServerOption

		grpcServer := grpc.NewServer(opts...)

		listener, err := net.Listen("tcp", c.grpcAddress)
		if err != nil {
			return err
		}

		log.Println("Starting gRPC server on address", grpcAddress)

		asset_service.RegisterAssetServer(grpcServer, proxyCache, assetDownloader, c.accessLogger, c.errorLogger)

		h := health.NewServer()
		grpc_health_v1.RegisterHealthServer(grpcServer, h)
		h.SetServingStatus("/grpc.health.v1.Health/Check", grpc_health_v1.HealthCheckResponse_SERVING)

		return grpcServer.Serve(listener)
	})

	return servers.Wait()
}

func main() {
	flag.Parse()

	config := &Config{
		accessLogger: log.New(os.Stdout, "", log.LstdFlags),
		errorLogger:  log.New(os.Stderr, "", log.LstdFlags),
		grpcAddress:  net.JoinHostPort(*host, strconv.Itoa(*port)),
		proxyAddress: net.JoinHostPort(*proxyHost, strconv.Itoa(*proxyPort)),
	}

	err := run(config)
	if err != nil {
		log.Fatalf("Run server error: %v", err)
	}
}
