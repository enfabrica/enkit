package mserver

import (
	"github.com/enfabrica/enkit/machinist"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc"
)

func New(mods ...Modifier) (*server, error) {
	s := &server{
		killChannel: make(chan error),
		Controller: &Controller{
			connectedNodes: map[string]*Node{},
		},
		SharedFlags: &machinist.SharedFlags{},
	}
	for _, m := range mods {
		if err := m(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type server struct {
	insecure      bool
	runningServer *grpc.Server
	killChannel   chan error

	Controller    *Controller
	*machinist.SharedFlags
}

func (s *server) MachinistFlags() *machinist.SharedFlags {
	return s.SharedFlags
}

func (s *server) Run() error {
	grpcs := grpc.NewServer()
	machinist_rpc.RegisterControllerServer(grpcs, s.Controller)
	s.runningServer = grpcs
	go func() {
		s.killChannel <- s.Controller.dnsServer.Run()
	}()

	return grpcs.Serve(s.Listener)
}

func (s *server) Stop() error {
	s.runningServer.Stop()
	return nil
}
