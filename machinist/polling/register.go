package polling

import (
	"context"
	"github.com/enfabrica/enkit/machinist/config"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"time"
)

// SendRegisterRequests is a blocking function that will send re-register requests every 5 seconds.
func SendRegisterRequests(ctx context.Context, client machinist_rpc.ControllerClient, state config.Node) error {
	pollStream, err := client.Poll(ctx)
	if err != nil {
		return err
	}
	registerRequest := &machinist_rpc.PollRequest{
		Req: &machinist_rpc.PollRequest_Register{
			Register: &machinist_rpc.ClientRegister{
				Name: state.Name,
				Tag:  state.Tags,
				Ips:  state.IpsAsString(),
			},
		},
	}
	for {
		_ = <-time.After(5 * time.Second)
		if err := pollStream.Send(registerRequest); err != nil {
			state.Errorf("unable to send request: %w", err)
		}
	}
}
