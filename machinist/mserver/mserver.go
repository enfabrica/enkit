package mserver

import (
	"github.com/enfabrica/enkit/machinist/config"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc"
)

func New(mods ...Modifier) (*ControlPlane, error) {
	s := &ControlPlane{
		killChannel: make(chan error),
		Controller: &Controller{
			connectedNodes: map[string]*Node{},
		},
		Common: config.DefaultCommonFlags(),
	}
	for _, m := range mods {
		if err := m(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type ControlPlane struct {
	insecure      bool
	runningServer *grpc.Server
	killChannel   chan error

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
	}()
	go s.Controller.ServeAllRecords()
	return grpcs.Serve(s.Listener)
}

func (s *ControlPlane) Stop() error {
	s.runningServer.Stop()
	return s.Controller.dnsServer.Stop()
}
