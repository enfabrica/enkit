package mnode

import (
	"context"
	"github.com/enfabrica/enkit/machinist"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"time"
)

type Node struct {
	Client machinist_rpc.ControllerClient
	*machinist.SharedFlags
	Name string
	Tags []string
	// Dial func will override any existing options to connect
	DialFunc func() (*grpc.ClientConn, error)
}

func (n *Node) Flags() *machinist.SharedFlags {
	return n.SharedFlags
}

func (n *Node) Init() error {
	if n.DialFunc != nil {
		conn, err := n.DialFunc()
		if err != nil {
			return err
		}
		n.Client = machinist_rpc.NewControllerClient(conn)
		return nil
	}
	panic("not implemented yet")
}

func (n *Node) BeginPolling() error {
	ctx := context.Background()
	pollStream, err := n.Client.Poll(ctx)
	if err != nil {
		return err
	}
	initialRequest := &machinist_rpc.PollRequest{
		Req: &machinist_rpc.PollRequest_Register{
			Register: &machinist_rpc.ClientRegister{
				Name: n.Name,
				Tag:  n.Tags,
			},
		},
	}
	if err := pollStream.Send(initialRequest); err != nil {
		return errors.Wrap(err, "unable to send initial request")
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
				return errors.Wrap(err, "unable to send poll req")
			}
		}
	}
}
