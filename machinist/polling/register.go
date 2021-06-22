package polling

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/machinist/config"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"time"
)

// SendRegisterRequests is a blocking function that will send re-register requests every 5 seconds.
func SendRegisterRequests(ctx context.Context, client machinist_rpc.ControllerClient, conf *config.Node) error {
	fmt.Println("client", client)
	pollStream, err := client.Poll(ctx)
	if err != nil {
		return err
	}
	l := conf.Common.Root.Log
	registerRequest := &machinist_rpc.PollRequest{
		Req: &machinist_rpc.PollRequest_Register{
			Register: &machinist_rpc.ClientRegister{
				Name: conf.Name,
				Tag:  conf.Tags,
				Ips:  conf.IpAddresses,
			},
		},
	}
	for {
		if err := pollStream.Send(registerRequest); err != nil {
			l.Errorf("unable to send request: %w", err)
		}
		_ = <-time.After(5 * time.Second)
	}
}
