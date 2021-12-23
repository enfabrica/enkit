package main

import (
	"fmt"
	"github.com/enfabrica/enkit/gee/cmd"
	"github.com/enfabrica/enkit/gee/lib"
	"os"
)

func main() {
	err := lib.Logger().OpenFile("/tmp/gee.log")
	if err != nil {
		fmt.Println("%q", err)
		os.Exit(1)
	}
	lib.Logger().Tracef("---")
	lib.Logger().Tracef("Invoked %q", os.Args)
	cmd.Execute()
}
