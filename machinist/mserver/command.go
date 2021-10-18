package mserver

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/spf13/cobra"
	"net"
	"strconv"
)

type controlPlaneFlags struct {
	Port      int
	DnsPort   int
	Domains   []string
	BindNet   string
	StateFile string
	bf        *client.BaseFlags
}

func NewCommand(bf *client.BaseFlags) *cobra.Command {
	cpf := &controlPlaneFlags{
		bf: bf,
	}
	c := &cobra.Command{
		Use: "controlplane",
		RunE: func(cmd *cobra.Command, args []string) error {
			dnsListener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cpf.DnsPort)))
			if err != nil {
				return err
			}
			machinistListener, err := net.Listen("tcp", net.JoinHostPort(cpf.BindNet, strconv.Itoa(cpf.Port)))
			if err != nil {
				return err
			}
			mController, err := NewController(
				WithKDnsFlags(
					kdns.WithTCPListener(dnsListener),
					kdns.WithDomains(cpf.Domains),
				),
			)
			if err != nil {
				return err
			}
			s, err := New(
				WithController(mController),
				WithMachinistFlags(
					config.WithInsecure(),
					config.WithListener(machinistListener),
				))
			if err != nil {
				return err
			}
			defer s.Stop()
			bf.Log.Infof("Running ControlPlane now")
			return s.Run()
		},
	}

	c.PersistentFlags().IntVar(&cpf.Port, "port", 8081, "Port that machinist will run on to interface between its nodes")
	c.PersistentFlags().IntVar(&cpf.DnsPort, "dns-port", 5353, "the udp port that the dns will be served on, also note it will also allocate the tcp socket on it as well")
	c.PersistentFlags().StringSliceVar(&cpf.Domains, "domains", []string{}, "domains that the master ControlPlane will be serving")
	c.PersistentFlags().StringVar(&cpf.BindNet, "bind-net", "127.0.0.1", "the address to bind the grpc listener to")
	c.PersistentFlags().StringVar(&cpf.StateFile, "state", "./state.json", "file to write and load state to")
	return c
}
