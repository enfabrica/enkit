package fusecmd

import (
	"errors"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	c := &cobra.Command{
		Use: "fuse",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("FUSE is not supported on this platform")
		},
	}
	return c
}
