package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	config := &Config{
		Name:     "hello",
		Tags:     []string{},
		DnsNames: []string{},
		af:       client.DefaultAuthFlags(),
		ms:       &machinist.SharedFlags{},
	}
	c := &cobra.Command{
		Use: "node [OPTIONS] [SUBCOMMANDS]",
	}
	fFunc := func() (*Node, error) {
		newN, err := New(config)
		if err != nil {
			return nil, err
		}
		return newN, err
	}
	kflags := &kcobra.FlagSet{FlagSet: c.PersistentFlags()}
	config.af.Register(kflags, "node-")
	c.PersistentFlags().StringVar(&config.CaPublicKeyLocation, "ca-key-file", "/etc/ssh/machinist_ca.pub", "the file location of the CA's public key from the auth server. If the file already exists, defers to the rewrite flag")
	c.PersistentFlags().StringVar(&config.HostKeyLocation, "host-key-file", "/etc/ssh/machinist_host_key", "the location where to save the machinist host key")
	c.PersistentFlags().StringVar(&config.SSHDConfigurationLocation, "sshd-configuration-file", "/etc/ssh/sshd_config.d/machinist.conf", "the location where to save the machinist host key")
	c.PersistentFlags().BoolVar(&config.AutoRestartSSHD, "auto-restart-ssh", true, "if enroll is is successful, auto restart sshd by calling service sshd-restart")
	c.PersistentFlags().BoolVar(&config.ReWriteConfigs, "rewrite", true, "rewrite HostKey and HostCert and TrustedCAKey if it already exists on the system")
	c.AddCommand(NewEnrollCommand(fFunc))
	return c
}

func NewEnrollCommand(factoryFunc FactoryFunc) *cobra.Command {
	c := &cobra.Command{
		Use:  "enroll [username] [OPTIONS]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := factoryFunc()
			if err != nil {
				return err
			}
			return n.Enroll(args[0])
		},
	}
	return c
}
