package main

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist/mnode"
)

func main() {
	base := client.DefaultBaseFlags("node", "enkit")
	node := mnode.NewRootCommand()

	set, populator, runner := kcobra.Runner(node, nil, base.IdentityErrorHandler("enkit login"))

	base.Run(set, populator, runner)
}
