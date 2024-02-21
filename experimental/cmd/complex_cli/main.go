package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

type PrintNum struct {
	*cobra.Command
	base int
}

func NewPrintNum() *PrintNum {
	c := &PrintNum{
		Command: &cobra.Command{
			Use:   "print-num",
			Short: "Print a number",
			Long:  `Print a number with various options`,
			Args:  cobra.ExactArgs(1),
		},
	}

	c.Command.PreRunE = c.Check
	c.Command.RunE = c.Run
	c.Flags().IntVar(&c.base, "base", 10, "Interpret the number as being in a specific base")

	return c
}

func (c *PrintNum) Check(cmd *cobra.Command, args []string) error {
	validBases := map[int]struct{}{
		2:  {},
		8:  {},
		10: {},
		16: {},
	}

	if _, ok := validBases[c.base]; !ok {
		return fmt.Errorf("%d is not a valid base", c.base)
	}
	return nil
}

func (c *PrintNum) Run(cmd *cobra.Command, args []string) error {
	num, err := strconv.ParseInt(args[0], c.base, 64)
	if err != nil {
		return fmt.Errorf("%q is not a valid number in base %d: %w", args[0], c.base, err)
	}
	fmt.Printf("%d\n", num)
	return nil
}

func main() {
	root := &cobra.Command{
		Use:   "complex_cli",
		Short: "Example of a CLI app with multiple subcommands",
	}

	root.AddCommand(NewPrintNum().Command)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Println("complex_cli success")
	}
}
