package enfuse

import (
	"github.com/spf13/cobra"
)

func NewFuseShareCommand() *cobra.Command {
	cc := &ConnectConfig{}
	var dir string
	c := &cobra.Command{
		Use: `share`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ServeDirectory(
				WithConnectMods(
					WithConnectConfig(cc),
				),
				WithDir(dir),
			)
		},
	}
	c.Flags().StringVar(&dir, "dir", ".", "the directory to share")
	c.Flags().IntVarP(&cc.Port, "port", "p", 9999, "the port to serve the rpc from")
	c.Flags().StringVarP(&cc.Url, "interface", "i", "127.0.0.1", "the interface to bind")
	return c
}

func NewFuseMountDirectory() *cobra.Command {
	cc := &ConnectConfig{}
	var cwd string
	c := &cobra.Command{
		Use: `mount`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fc, err := NewClient(cc)
			if err != nil {
				return err
			}
			return MountDirectory(cwd, fc)
		},
	}
	c.Flags().StringVar(&cwd, "dir", ".", "the mount point for the FUSE directory")
	c.Flags().IntVarP(&cc.Port, "port", "p", 9999, "the port to serve the rpc from")
	c.Flags().StringVarP(&cc.Url, "interface", "i", "127.0.0.1", "the interface to bind to")
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
