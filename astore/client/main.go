package main

import (
	acommands "github.com/enfabrica/enkit/astore/client/commands"
	bcommands "github.com/enfabrica/enkit/lib/client/commands"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"

	"github.com/enfabrica/enkit/lib/srand"
	"math/rand"
)

func main() {
	rng := rand.New(srand.Source)

	base := bcommands.NewBase()
	root := acommands.New(rng, base)
	base.Register(root.PersistentFlags())

	root.AddCommand(bcommands.NewLogin(base, root.Command.Name(), rng).Command)

	kcobra.RunWithDefaults(root.Command, &base.Populator, &base.Log)
}
