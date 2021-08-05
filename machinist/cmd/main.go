package main

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/machinist"
)

func main() {
	base := client.DefaultBaseFlags("astore", "enkit")
	c := machinist.NewRootCommand(base)

	set, populator, runner := kcobra.Runner(c, nil, base.IdentityErrorHandler("enkit login"))

	base.Run(set, populator, runner)
}
