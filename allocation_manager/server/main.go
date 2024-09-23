package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	//"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	//	"github.com/enfabrica/enkit/allocation_manager/frontend"
	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/allocation_manager/service"
	//"github.com/enfabrica/enkit/lib/metrics"
	"github.com/enfabrica/enkit/lib/server"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	//go:embed templates/*
	templates     embed.FS
	serviceConfig = flag.String("service_config", "", "Path to service configuration textproto")
)

func exitIf(err error) {
	if err != nil {
		// TODO: Use enkit logging enkit/lib/logger/logger.go package logger... "github.com/enfabrica/enkit/lib/logger/logger"
		log.Fatal(err)
	}
}

func checkFlags() error {
	if *serviceConfig == "" {
		return fmt.Errorf("--service_config must be provided")
	}
	return nil
}

func loadConfig(path string) (*apb.Config, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config %q: %w", path, err)
	}
	var config apb.Config
	err = prototext.Unmarshal(contents, &config)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config %q: %w", path, err)
	}
	return &config, nil
}

func main() {
	ctx := context.Background()
	// TODO: Use enkit flag libraries
	flag.Parse()
	exitIf(checkFlags())

	config, err := loadConfig(*serviceConfig)
	exitIf(err)

	//	template, err := template.ParseFS(templates, "**/*.tmpl")
	//	exitIf(err)

	grpcs := grpc.NewServer()
	s, err := service.New(config)
	exitIf(err)
	apb.RegisterAllocationManagerServer(grpcs, s)

	//	fe := frontend.New(template, s)

	mux := http.NewServeMux()
	//	metrics.AddHandler(mux, "/metrics")
	//	mux.Handle("/queue", fe)

  // port from https://docs.google.com/document/d/1ZtmR60B-pBRlTQSw_aqaujUOWe6tD6TTNbNj7VdZHAY/edit
  lis, err := net.Listen("tcp", ":6435")
	exitIf(err)

	exitIf(server.Run(ctx, mux, grpcs, lis))
}
