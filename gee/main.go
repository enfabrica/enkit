package main

import (
	commands "github.com/enfabrica/gee/commands"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
)

func main() {
	base := client.DefaultBaseFlags("gee", "enkit")
	root := commands.New(base)

	set, populator, runner := kcobra.Runner(root.Command, nil, base.IdentityErrorHandler("gee login"))

	base.Run(set, populator, runner)
}
