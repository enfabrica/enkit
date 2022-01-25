package mserver

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"github.com/miekg/dns"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"time"
)

type Controller struct {
	Log logger.Logger

	startUpFunc []func()

	allRecordsRefreshRate time.Duration

	State         *state.MachineController
	stateFile     string
	stateWriteTTL time.Duration

	dnsServer *kdns.DnsServer
	domains   []string
}

// Init is designed to run after all components have been started up before running itself as a server
func (en *Controller) Init() {
	for _, m := range en.State.Machines {
		en.addNodeToDns(m.Name, m.Ips, m.Tags)
	}
}

func (en *Controller) Nodes() []*state.Machine {
	en.State.Lock()
	defer en.State.Unlock()
	nodes := make([]*state.Machine, len(en.State.Machines))
	copy(nodes, en.State.Machines)
	return nodes
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
	newMachine := &state.Machine{
		Name: ping.Name,
		Ips:  parsedIps,
		Tags: ping.Tag,
	}
	if err := state.AddMachine(en.State, newMachine); err != nil {
		return status.Errorf(codes.AlreadyExists, err.Error())
	}
	en.addNodeToDns(ping.Name, newMachine.Ips, newMachine.Tags)
	return stream.Send(
		&machinist.PollResponse{
			Resp: &machinist.PollResponse_Result{
				Result: &machinist.ActionResult{},
			},
		})

}

func (en *Controller) Poll(stream machinist.Controller_PollServer) error {
	for {
		in, err := stream.Recv()
		if err != nil {
			return err
		}

		switch r := in.Req.(type) {
		case *machinist.PollRequest_Ping:
			en.HandlePing(stream, r.Ping)

		case *machinist.PollRequest_Register:
			if err = en.HandleRegister(stream, r.Register); err != nil {
				return err
			}
		}
	}
}

func (en *Controller) addNodeToDns(name string, ips []net.IP, tags []string) {
	for _, d := range en.dnsServer.Domains {
		dnsName := dns.CanonicalName(fmt.Sprintf("%s.%s", name, d))
		var recordTags []dns.RR
		for _, t := range tags {
			entry, err := dns.NewRR(fmt.Sprintf("%s %s %s", dnsName, "TXT", t))
			if err != nil {
				continue
			}
			recordTags = append(recordTags, entry)
		}
		for _, i := range ips {
			var recordType string
			if i.To4() != nil {
				recordType = "A"
			}
			if i.To16() != nil && recordType == "" {
				recordType = "AAAA"
			}
			if recordType != "" {
				entry, err := dns.NewRR(fmt.Sprintf("%s %s %s", dnsName, recordType, i.String()))
				if err != nil {
					continue
				}
				en.Log.Infof("Adding %s to the dns ControlPlane %s", dnsName, entry)
				en.dnsServer.SetEntry(dnsName, []dns.RR{entry})
				en.dnsServer.SetEntry(dnsName, recordTags)
			}
		}
	}
}

// ServeAllAndInfoRecords will continuously poll Nodes() and create multiple _all.<domain> records containing the ip addresses
// of all machines attached.
// TODO(adam): be able to pass in a wrapped ticker for testing intervals
func (en *Controller) ServeAllAndInfoRecords(killChannel chan struct{}, killChannelAck chan struct{}) {
	for {
		select {
		case <-time.After(en.allRecordsRefreshRate):
			ns := en.Nodes()
			for _, d := range en.dnsServer.Domains {
				dnsName := dns.CanonicalName(fmt.Sprintf("%s.%s", "_all", d))
				infoDnsName := dns.CanonicalName(fmt.Sprintf("%s.%s", "_info", d))
				var allDnsRecords []dns.RR
				var infoDnsRecords []dns.RR
				for _, v := range ns {
					for _, i := range v.Ips {
						rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", dnsName, "A", i.String()))
						if err != nil {
							en.Log.Errorf("err: %v", err)
						}
						infoRR, err := dns.NewRR(fmt.Sprintf("%s %s { name: %s, ip: %s }", infoDnsName, "TXT", v.Name, i.String()))
						if err != nil {
							en.Log.Errorf("err: %v", err)
						}
						allDnsRecords = append(allDnsRecords, rr)
						infoDnsRecords = append(infoDnsRecords, infoRR)
					}
				}
				en.dnsServer.SetEntry(dnsName, allDnsRecords)
				en.dnsServer.SetEntry(infoDnsName, infoDnsRecords)
			}
		case <-killChannel:
			killChannelAck <- struct{}{}
			return
		}
	}
}

// WriteState writes state to the specified state file every 2 seconds. Will not exit or error out unless no statefile is
// provided.
func (en *Controller) WriteState() {
	if en.stateFile == "" {
		en.Log.Warnf("No path to state provided, state is fully in memory")
		return
	}
	for {
		<-time.After(en.stateWriteTTL)
		if err := state.WriteController(en.State, en.stateFile); err != nil {
			en.Log.Errorf("machinist: writing to state failed with err: %v", err)
		}
	}
}
