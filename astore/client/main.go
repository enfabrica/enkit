package main

import (
	acommands "github.com/enfabrica/enkit/astore/client/commands"
	bcommands "github.com/enfabrica/enkit/lib/client/commands"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"

	"github.com/enfabrica/enkit/lib/srand"
	"math/rand"
)

func main() {
	base := client.DefaultBaseFlags("astore", "enkit")
	root := acommands.New(base)

	set, populator, runner := kcobra.Runner(root.Command, nil, client.HandleIdentityError("astore login youruser@yourdomain.com"))

	rng := rand.New(srand.Source)
	root.AddCommand(bcommands.NewLogin(base, rng, populator).Command)

	base.Run(set, populator, runner)
}
