package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	var n *Node
	nf := NodeFlags{
		af: client.DefaultAuthFlags(),
		ms: &machinist.SharedFlags{},
	}
	c := &cobra.Command{
		Use: "node [OPTIONS] [SUBCOMMANDS]",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			newN, err := New(nf.ToModifiers()...)
			if err != nil {
				return err
			}
			n = newN
			return nil
		},
	}
	kflags := &kcobra.FlagSet{FlagSet: c.PersistentFlags()}
	nf.af.Register(kflags, "")
	c.AddCommand(NewEnrollCommand(n))
	return c
}

func NewEnrollCommand(n *Node) *cobra.Command {
	c := &cobra.Command{
		Use:  "enroll [username] [OPTIONS]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return n.Enroll(args[0])
		},
	}
	return c
}
