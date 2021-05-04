package mserver

import (
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"log"
	"sync"
)

type Controller struct {
	Log logger.Logger

	domains             []string
	connectedNodes      map[string]*Node
	connectedNodesMutex sync.RWMutex

	dnsServer *kdns.DnsServer
}

func (en *Controller) Nodes() []*Node {
	en.connectedNodesMutex.RLock()
	defer en.connectedNodesMutex.RUnlock()
	var nodes []*Node
	for _, v := range en.connectedNodes {
		nodes = append(nodes, v)
	}
	return nodes
}

func (en *Controller) Node(name string) *Node {
	en.connectedNodesMutex.RLock()
	defer en.connectedNodesMutex.RUnlock()
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
	n := &Node{
		Name: ping.Name,
		Tags: ping.Tag,
	}
	en.connectedNodesMutex.Lock()
	en.connectedNodes[ping.Name] = n
	en.connectedNodesMutex.Unlock()
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

func (en *Controller) ServeDns() error {
	dnsServ, err := kdns.NewDNS(kdns.WithPort(5553),
		kdns.WithHost("127.0.0.1"),
		kdns.WithLogger(en.Log),
		kdns.WithDomains(en.domains))
	if err != nil {
		return err
	}
	en.dnsServer = dnsServ
	return dnsServ.Run()
}

func (en *Controller) addNodeToDns(name string, ips []string)  {
	for _, d := range en.domains {

	}
}
