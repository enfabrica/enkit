package mserver

import (
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"log"
	"sync"
)

type Controller struct {
	connectedNodes map[string]*Node
	sync.RWMutex
}

func (en *Controller) Nodes() []*Node {
	var nodes []*Node
	for _, v := range en.connectedNodes {
		nodes = append(nodes, v)
	}
	return nodes
}

func (en *Controller) Node(name string) *Node {
	return en.connectedNodes[name]
}

func (en *Controller) Download(*machinist.DownloadRequest, machinist.Controller_DownloadServer) error {
	return nil
}

func (en *Controller) Upload(machinist.Controller_UploadServer) error {
	return nil
}

func (en *Controller) HandlePing(stream machinist.Controller_PollServer, ping *machinist.ClientPing) error {
	return stream.Send(
		&machinist.PollResponse{
			Resp: &machinist.PollResponse_Pong{
				Pong: &machinist.ActionPong{
					Payload: ping.Payload,
				},
			},
		})

}

func (en *Controller) HandleRegister(stream machinist.Controller_PollServer, ping *machinist.ClientRegister) error {
	en.Lock()
	n := &Node{
		Name: ping.Name,
		Tags: ping.Tag,
	}
	en.connectedNodes[ping.Name] = n
	en.Unlock()
	return stream.Send(
		&machinist.PollResponse{
			Resp: &machinist.PollResponse_Result{
				Result: &machinist.ActionResult{

				},
			},
		})

}

func (en *Controller) Poll(stream machinist.Controller_PollServer) error {
	for {
		in, err := stream.Recv()
		if err != nil {
			return err
		}
		log.Printf("GOT %#v", in.Req)

		switch r := in.Req.(type) {
		case *machinist.PollRequest_Ping:
			en.HandlePing(stream, r.Ping)

		case *machinist.PollRequest_Register:
			en.HandleRegister(stream, r.Register)
			log.Printf("Got REGISTER %#v", *r.Register)
		}
	}
}
