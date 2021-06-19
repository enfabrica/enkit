package polling

import (
	"context"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"time"
)
// SendKeepAliveRequest will run a keepalive request ad infinittum, only logging when EOF.
func SendKeepAliveRequest(ctx context.Context, client machinist_rpc.ControllerClient) error {
	pollStream, err := client.Poll(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-time.After(1 * time.Second):
			pollReq := &machinist_rpc.PollRequest{
				Req: &machinist_rpc.PollRequest_Ping{
					Ping: &machinist_rpc.ClientPing{
						Payload: []byte(``),
					},
				},
			}
			if err := pollStream.Send(pollReq); err != nil {
				ps, err := client.Poll(ctx)
				if err != nil {
					continue
				}
				pollStream = ps
			}
		}
	}
}
