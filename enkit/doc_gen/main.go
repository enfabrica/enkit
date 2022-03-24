package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/enfabrica/enkit/enkit/cmd"
)

var (
	outDir = flag.String("out-dir", "", "Path to output directory to emit markdown files")
)

func main() {
	flag.Parse()

	command, err := cmd.New()
	exitIf(err)

	err = command.GenMarkdownTree(*outDir)
	exitIf(err)
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
