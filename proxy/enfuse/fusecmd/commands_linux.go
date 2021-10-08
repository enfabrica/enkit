package fusecmd

import (
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/spf13/cobra"
)

func NewFuseShareCommand() *cobra.Command {
	cc := &enfuse.ConnectConfig{}
	var dir string
	c := &cobra.Command{
		Use: `share`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return enfuse.ServeDirectory(
				enfuse.WithConnectMods(
					enfuse.WithConnectConfig(cc),
				),
				enfuse.WithDir(dir),
			)
		},
	}
	c.Flags().StringVar(&dir, "dir", ".", "the directory to share")
	c.Flags().IntVarP(&cc.Port, "port", "p", 9999, "the port to serve the rpc from")
	c.Flags().StringVarP(&cc.Url, "interface", "i", "127.0.0.1", "the interface to bind")
	return c
}

func NewFuseMountDirectory() *cobra.Command {
	cc := &enfuse.ConnectConfig{}
	var cwd string
	c := &cobra.Command{
		Use: `mount`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fc, err := enfuse.NewClient(cc)
			if err != nil {
				return err
			}
			return enfuse.MountDirectory(cwd, fc)
		},
	}
	c.Flags().StringVar(&cwd, "dir", ".", "the mount point for the FUSE directory")
	c.Flags().IntVarP(&cc.Port, "port", "p", 9999, "the port to serve the rpc from")
	c.Flags().StringVarP(&cc.Url, "interface", "i", "127.0.0.1", "the interface to bind to")
	return c
}

func New() *cobra.Command {
	c := &cobra.Command{
		Use: "fuse",
	}
	c.AddCommand(NewFuseMountDirectory())
	c.AddCommand(NewFuseShareCommand())
	return c
}
