package main

import (
	"fmt"
	"log"
	"os"

	bbclientdexec "github.com/enfabrica/enkit/lib/kbuildbarn/exec"
	"github.com/enfabrica/enkit/lib/logger"
)

func main() {
	opts := bbclientdexec.NewClientOptions(&logger.DefaultLogger{Printer: log.Printf}, 8866, "/home/bminor/bbtest2")
	c, err := bbclientdexec.MaybeStartClient(opts)
	exitIf(err)
	c = c
}

func exitIf(err error) {
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
}
