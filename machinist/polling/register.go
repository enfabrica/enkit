package polling

import (
	"context"
	"github.com/enfabrica/enkit/machinist/config"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc/status"
	"time"
)

// SendRegisterRequests is a blocking function that will send re-register requests every 5 seconds.
func SendRegisterRequests(ctx context.Context, client machinist_rpc.ControllerClient, conf *config.Node) error {
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
			_, err := pollStream.Recv()
			s, ok := status.FromError(err)
			if ok {
				l.Errorf("unable to send register request: %+v", s)
			} else {
				l.Errorf("unable to send request, unknown err: %w", err)
			}
			p, err := client.Poll(ctx)
			if err != nil {
				l.Errorf("error %w reconnecting, trying again", err)
				registerFailCounter.Inc()
			} else {
				l.Infof("Successfully reconnected")
				pollStream = p
			}
		}
		_ = <-time.After(5 * time.Second)
	}
}
