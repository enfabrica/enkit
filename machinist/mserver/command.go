package mserver

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/machinist"
	"github.com/spf13/cobra"
	"net"
	"strconv"
)

type controlPlaneFlags struct {
	Port     int
	DnsPort  int
	Domains  []string
	BindPort string
	bf       *client.BaseFlags
}

func NewCommand(bf *client.BaseFlags) *cobra.Command {
	config := &controlPlaneFlags{
		bf: bf,
	}
	c := &cobra.Command{
		Use: "controlplane",
		RunE: func(cmd *cobra.Command, args []string) error {
			dnsListener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(config.DnsPort)))
			if err != nil {
				return err
			}
			machinistListener, err := net.Listen("tcp", net.JoinHostPort(config.BindPort, strconv.Itoa(config.Port)))
			if err != nil {
				return err
			}
			mController, err := NewController(
				DnsPort(config.Port),
				WithKDnsFlags(
					kdns.WithListener(dnsListener),
					kdns.WithDomains(config.Domains),
				),
			)
			if err != nil {
				return err
			}
			s, err := New(
				WithController(mController),
				WithMachinistFlags(
					machinist.WithInsecure(),
					machinist.WithListener(machinistListener),
				))
			if err != nil {
				return err
			}
			defer s.Stop()
			bf.Log.Infof("Running ControlPlane now")
			return s.Run()
		},
	}

	c.PersistentFlags().IntVar(&config.Port, "port", 8081, "Port that machinist will run on to interface between its nodes")
	c.PersistentFlags().IntVar(&config.DnsPort, "dns-port", 5353, "the udp port that the dns will be served on, also note it will also allocate the tcp socket on it as well")
	c.PersistentFlags().StringSliceVar(&config.Domains, "domains", []string{}, "domains that the master ControlPlane will be serving")
	c.PersistentFlags().StringVar(&config.BindPort, "bind-net", "127.0.0.1", "the address to bind the grpc listener to")
	return c
}
