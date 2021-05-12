package main

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/spf13/cobra"
)

func main() {
	c := &cobra.Command{Use: "machinist"}
	base := client.DefaultBaseFlags("astore", "enkit")

	node := mnode.NewRootCommand(base)
	controlplane := mserver.NewCommand(base)
	c.AddCommand(node)
	c.AddCommand(controlplane)

	set, populator, runner := kcobra.Runner(node, nil, base.IdentityErrorHandler("enkit login"))

	base.Run(set, populator, runner)
}
