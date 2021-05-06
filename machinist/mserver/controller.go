package mserver

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/miekg/dns"
	"log"
	"net"
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
	var parsedIps []net.IP
	for _, p := range ping.Ips {
		i := net.ParseIP(p)
		if i != nil {
			parsedIps = append(parsedIps, i)
		}
	}
	if len(parsedIps) == 0 {
		return errors.New("no valid ip sent")
	}
	n := &Node{
		Name: ping.Name,
		Tags: ping.Tag,
		Ips:  parsedIps,
	}
	en.connectedNodesMutex.Lock()
	en.connectedNodes[ping.Name] = n
	en.connectedNodesMutex.Unlock()
	en.addNodeToDns(ping.Name, n.Ips)
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
			if err = en.HandleRegister(stream, r.Register); err != nil {
				return err
			}
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

func (en *Controller) addNodeToDns(name string, ips []net.IP) {
	for _, d := range en.domains {
		dnsName := dns.CanonicalName(fmt.Sprintf("%s.%s", name, d))
		for _, i := range ips {
			var recordType string
			if i.To4() != nil {
				recordType = "A"
			}
			if i.To16() != nil {
				recordType = "AAAA"
			}
			if recordType != "" {
				entry, err := dns.NewRR(fmt.Sprintf("%s %s %s", dnsName, recordType, i.String()))
				if err != nil {
					continue
				}
				en.dnsServer.AddEntry(dnsName, entry)
			}
		}
	}
}
