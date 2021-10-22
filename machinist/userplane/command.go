package userplane

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	c := &cobra.Command{
		Use: `userplane`,
	}
	c.AddCommand(NewServeCommand())
	return c
}

func NewServeCommand() *cobra.Command{
	c := &cobra.Command{
		Use: "serve",
	}
	return c
}

