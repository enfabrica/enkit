package main

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist/mnode"
)

func main() {
	base := client.DefaultBaseFlags("enkit", "enkit")
	node := mnode.NewRootCommand(base)

	set, populator, runner := kcobra.Runner(node, nil, base.IdentityErrorHandler("enkit login"))

	base.Run(kcobra.HideFlags(set), populator, runner)
}
