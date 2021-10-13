package mserver

import (
	"fmt"
	"github.com/enfabrica/enkit/machinist/config"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc"
)

func New(mods ...Modifier) (*ControlPlane, error) {
	kd, err := NewController()
	if err != nil {
		return nil, err
	}
	s := &ControlPlane{
		killChannel:           make(chan error),
		Controller:            kd,
		Common:                config.DefaultCommonFlags(),
		allRecordsKillChannel: make(chan struct{}, 2),
	}
	for _, m := range mods {
		if err := m(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type ControlPlane struct {
	insecure              bool
	runningServer         *grpc.Server
	allRecordsKillChannel chan struct{}
	killChannel           chan error

	Controller *Controller
	*config.Common
}

func (s *ControlPlane) MachinistCommon() *config.Common {
	return s.Common
}

func (s *ControlPlane) Run() error {
	grpcs := grpc.NewServer()
	machinist_rpc.RegisterControllerServer(grpcs, s.Controller)
	s.runningServer = grpcs
	go func() {
		s.killChannel <- s.Controller.dnsServer.Run()
		fmt.Println("it ended")
	}()
	s.Controller.Init()
	go s.Controller.ServeAllRecords(s.allRecordsKillChannel)
	go s.Controller.WriteState()
	return grpcs.Serve(s.Listener)
}

func (s *ControlPlane) Stop() error {
	s.allRecordsKillChannel <- struct{}{}
	<-s.allRecordsKillChannel
	s.runningServer.Stop()
	return s.Controller.dnsServer.Stop()
}
