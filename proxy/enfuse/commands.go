package enfuse

import (
	"github.com/spf13/cobra"
)

func NewFuseShareCommand() *cobra.Command {
	var port int
	var cwd string
	c := &cobra.Command{
		Use: `share`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ServeDirectory()
		},
	}
	c.Flags().StringVar(&cwd, "dir", ".", "")
	c.Flags().IntVarP(&port, "port", "p", 9999, "")
	return c
}

func NewFuseMountDirectory() *cobra.Command {
	var cwd string
	c := &cobra.Command{
		Use: `mount`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MountDirectory(cwd, &FuseClient{})
		},
	}
	c.Flags().StringVar(&cwd, "dir", ".", "")
	return c
}

func NewCommand() *cobra.Command {
	c := &cobra.Command{
		Use: "fuse",
	}
	c.AddCommand(NewFuseMountDirectory())
	c.AddCommand(NewFuseShareCommand())
	return c
}
