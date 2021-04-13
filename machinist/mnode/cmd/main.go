package main

import (
	"github.com/enfabrica/enkit/machinist/mnode"
	"log"
)

func main() {
	defer log.Fatal(mnode.NewRootCommand().Execute())
}
