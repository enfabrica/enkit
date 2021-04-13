package mnode

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"google.golang.org/grpc"
	"time"
)

type Node struct {
	MachinistClient machinist_rpc.ControllerClient
	AuthClient      auth.AuthClient
	Repeater        *retry.Options
	Log             logger.Logger

	// Dial func will override any existing options to connect
	DialFunc func() (*grpc.ClientConn, error)

	nf *NodeFlags
}

func (n *Node) Init() error {
	if n.DialFunc != nil {
		conn, err := n.DialFunc()
		if err != nil {
			return err
		}
		n.MachinistClient = machinist_rpc.NewControllerClient(conn)
		return nil
	}
	panic("not implemented yet")
}

func (n *Node) BeginPolling() error {
	ctx := context.Background()
	pollStream, err := n.MachinistClient.Poll(ctx)
	if err != nil {
		return err
	}
	initialRequest := &machinist_rpc.PollRequest{
		Req: &machinist_rpc.PollRequest_Register{
			Register: &machinist_rpc.ClientRegister{
				Name: n.nf.Name,
				Tag:  n.nf.Tags,
			},
		},
	}
	if err := pollStream.Send(initialRequest); err != nil {
		return fmt.Errorf("unable to send initial request: %w", err)
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
				return fmt.Errorf("unable to send poll req: %w", err)
			}
		}
	}
}
