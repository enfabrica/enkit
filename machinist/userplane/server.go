package userplane

import (
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"google.golang.org/grpc"
)

type Server struct {
	c             *Controller
	config        *Config
	runningServer *grpc.Server
}

func (s *Server) Serve() error {
	grpcs := grpc.NewServer()
	s.runningServer = grpcs
	c := &Controller{
		State: &state.UserPlane{},
	}
	machinist.RegisterUserPlaneServer(grpcs, c)
	machinist.RegisterUserplaneStateServer(grpcs, c)
	return grpcs.Serve(s.config.Lis)
}
func (s Server) Stop() error {
	s.runningServer.Stop()
	return nil
}

func NewServer(mods ...ConfigMod) (*Server, error) {
	c := &Config{}
	for _, m := range mods {
		c = m(c)
	}
	if err := c.Verify(); err != nil {
		return nil, err
	}
	return &Server{
		c: &Controller{
			State: &state.UserPlane{},
		},
		config: c,
	}, nil
}
