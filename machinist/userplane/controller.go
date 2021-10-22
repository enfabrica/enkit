package userplane

import (
	"context"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
)

var (
	_ machinist.UserplaneStateServer = &Controller{}
	_ machinist.UserPlaneServer      = &Controller{}
)

type Controller struct {
	State *state.UserPlane
}

func (c *Controller) List(ctx context.Context, request *machinist.ListRequest) (*machinist.ListResponse, error) {
	var ums []*machinist.UserMachine
	for _, s := range c.State.Machines {
		ums = append(ums, &machinist.UserMachine{
			Name:        s.Name,
			Ips:         s.Ips,
			Alive:       true,
			Reservation: nil,
			Tags:        s.Tags,
		})
	}
	if request.Limit > 0 {
		ums = ums[0:request.Limit]
	}
	return &machinist.ListResponse{
		Machines: ums,
	}, nil
}

func (c *Controller) Tag(ctx context.Context, request *machinist.TagRequest) (*machinist.TagResponse, error) {
	panic("implement me")
}

func (c *Controller) ExportState(ctx context.Context, request *machinist.StateForwardRequest) (*machinist.StateForwardResponse, error) {
	state.MergeStates(c.State, request.Machines)
	return &machinist.StateForwardResponse{}, nil
}

func (c *Controller) Reserve(ctx context.Context, request *machinist.ReserveRequest) (*machinist.ReserveResponse, error) {
	panic("implement me")
}
