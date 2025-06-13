package main

import (
	"context"
	"embed"
	"flag"
	"fmt"

	"encoding/json"

	//"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	//	"github.com/enfabrica/enkit/allocation_manager/frontend"
	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/allocation_manager/service"

	//"github.com/enfabrica/enkit/lib/metrics"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/server"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	//go:embed templates/*
	templates     embed.FS
	serviceConfig = flag.String("service_config", "", "Path to service configuration textproto")
	hostInventory = flag.String("host_inventory", "", "Path to host inventory JSON file, as created by host_info.py script in internal/infra/allocation_manager")
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
	if *hostInventory == "" {
		return fmt.Errorf("--host_inventory must be provided")
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

func loadInventory(path string) (*apb.HostInventory, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read inventory from '%q': %w", path, err)
	}
	var inventory apb.HostInventory
	err = json.Unmarshal(contents, &inventory)
	if err != nil {
		return nil, fmt.Errorf("unable to parse inventory from '%q': %w", path, err)
	}
	return &inventory, nil
}

func printInventory(inventory *apb.HostInventory) {
	logger.Go.Infof("Host Inventory")
	for hostname, host := range inventory.GetHosts() {
		logger.Go.Infof("  %s	", hostname)
		
		logger.Go.Infof("    %d GPU(s)", len(host.GetGpuInfos()))
		if len(host.GetGpuInfos()) > 0 {
			// for _, gpu := range host.GetGpuInfos() {
			// 	logger.Go.Debugf("      %s => %s [%s]", gpu.GetBusId(), gpu.GetGpuModel(), gpu.GetCardModel(), )
			// }
		}

		logger.Go.Infof("    %d CPU(s)", len(host.GetCpuInfos()))
		if len(host.GetCpuInfos()) > 0 {
			// for _, cpu := range host.GetCpuInfos() {
			// 	logger.Go.Debugf("      %d => %fx%d [%s]", cpu.GetCpuIdx(), cpu.GetFreqMhz(), cpu.GetNumCores(), cpu.GetCache())
			// }
		}
	}
}

func main() {
	ctx := context.Background()
	// TODO: Use enkit flag libraries
	flag.Parse()
	exitIf(checkFlags())

	config, err := loadConfig(*serviceConfig)
	exitIf(err)

	inventory, err := loadInventory(*hostInventory)
	exitIf(err)

	printInventory(inventory)

	//	template, err := template.ParseFS(templates, "**/*.tmpl")
	//	exitIf(err)

	grpcs := grpc.NewServer()
	s, err := service.New(config, inventory)
	exitIf(err)
	apb.RegisterAllocationManagerServer(grpcs, s)

	//	fe := frontend.New(template, s)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	//	metrics.AddHandler(mux, "/metrics")
	//	mux.Handle("/queue", fe)

	// port from https://docs.google.com/document/d/1ZtmR60B-pBRlTQSw_aqaujUOWe6tD6TTNbNj7VdZHAY/edit
	lis, err := net.Listen("tcp", ":6435")
	exitIf(err)

	exitIf(server.Run(ctx, mux, grpcs, lis))
}
