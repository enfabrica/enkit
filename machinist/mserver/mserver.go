package mserver

import (
	"github.com/enfabrica/enkit/machinist"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc"
	"sync"
)

func New(mods ...Modifier) (*server, error) {
	s := &server{
		killChannel: make(chan error),
		Controller:  &Controller{},
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
	sync.Mutex
	*machinist.SharedFlags
}

func (s *server) Flags() *machinist.SharedFlags {
	return s.SharedFlags
}

func (s *server) Start() error {
	grpcs := grpc.NewServer()
	machinist_rpc.RegisterControllerServer(grpcs, s.Controller)
	s.runningServer = grpcs
	return grpcs.Serve(s.Listener)
}

func (s *server) Stop() error {
	s.runningServer.Stop()
	return nil
}
