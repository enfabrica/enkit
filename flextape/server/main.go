package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	fpb "github.com/enfabrica/enkit/flextape/proto"
	"github.com/enfabrica/enkit/flextape/service"
	"github.com/enfabrica/enkit/lib/server"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	serviceConfig = flag.String("service_config", "", "Path to service configuration textproto")
)

func exitIf(err error) {
	if err != nil {
		// TODO: Use enkit logging libraries
		log.Fatal(err)
	}
}

func checkFlags() error {
	if *serviceConfig == "" {
		return fmt.Errorf("--service_config must be provided")
	}
	return nil
}

func loadConfig(path string) (*fpb.Config, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config %q: %w", path, err)
	}
	var config fpb.Config
	err = prototext.Unmarshal(contents, &config)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config %q: %w", path, err)
	}
	return &config, nil
}

func main() {
	// TODO: Use enkit flag libraries
	flag.Parse()
	exitIf(checkFlags())

	config, err := loadConfig(*serviceConfig)
	exitIf(err)

	grpcs := grpc.NewServer()
	s := service.New(config)
	fpb.RegisterFlextapeServer(grpcs, s)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server.Run(mux, grpcs)
}
