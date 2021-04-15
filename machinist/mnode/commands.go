package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	nf := &NodeFlags{
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
		newN, err := New(nf)
		if err != nil {
			return nil, err
		}
		return newN, err
	}
	kflags := &kcobra.FlagSet{FlagSet: c.PersistentFlags()}
	nf.af.Register(kflags, "")
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
