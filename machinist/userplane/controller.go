package userplane

import (
	"context"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"net"
)

var (
	_ machinist.UserplaneStateServer = &Controller{}
	_ machinist.UserPlaneServer      = &Controller{}
)

type Controller struct {
	State *state.UserPlane
}

func ConvertIPS(sips []string) []net.IP {
	var toReturn []net.IP
	for _, si := range sips {
		toReturn = append(toReturn, net.ParseIP(si))
	}
	return toReturn
}

func (c *Controller) ImportState(ctx context.Context, request *machinist.StateForwardRequest) (*machinist.StateForwardResponse, error) {
	var lm []*state.Machine
	for _, m := range request.Machines {
		lm = append(lm, &state.Machine{
			Name: m.Name,
			Tags: m.Tags,
			Ips:  ConvertIPS(m.Ips),
		})
	}
	state.MergeStates(c.State, lm)
	return &machinist.StateForwardResponse{}, nil
}

func (c *Controller) Reserve(ctx context.Context, request *machinist.ReserveRequest) (*machinist.ReserveResponse, error) {
	panic("implement me")
}
