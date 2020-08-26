package main

import (
	"github.com/enfabrica/enkit/astore/client/commands"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"

	"github.com/enfabrica/enkit/lib/srand"
	"math/rand"
)

func main() {
	rng := rand.New(srand.Source)

	root := commands.NewRoot()

	root.AddCommand(commands.NewLogin(root, rng).Command)
	root.AddCommand(commands.NewDownload(root).Command)
	root.AddCommand(commands.NewUpload(root).Command)
	root.AddCommand(commands.NewList(root).Command)
	root.AddCommand(commands.NewGuess(root).Command)
	root.AddCommand(commands.NewTag(root).Command)
	root.AddCommand(commands.NewNote(root).Command)
	root.AddCommand(commands.NewPublic(root).Command)

	kcobra.RunWithDefaults(root.Command, &root.Populator, &root.Log)
}
