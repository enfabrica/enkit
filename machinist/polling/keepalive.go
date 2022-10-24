package polling

import (
	"context"

	mpb "github.com/enfabrica/enkit/machinist/rpc"

	"time"
)
// SendKeepAliveRequest will run a keepalive request ad infinittum, only logging when EOF.
func SendKeepAliveRequest(ctx context.Context, client mpb.ControllerClient) error {
	pollStream, err := client.Poll(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-time.After(1 * time.Second):
			pollReq := &mpb.PollRequest{
				Req: &mpb.PollRequest_Ping{
					Ping: &mpb.ClientPing{
						Payload: []byte(``),
					},
				},
			}
			if err := pollStream.Send(pollReq); err != nil {
				ps, err := client.Poll(ctx)
				if err != nil {
					keepAliveErrorCounter.Inc()
					continue
				}
				pollStream = ps
			}
		}
	}
}
