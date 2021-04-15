package mnode

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"log"
	"time"
)

type FactoryFunc = func() (*Node, error)

func New(nf *NodeFlags, mods ...NodeModifier) (*Node, error) {
	n := &Node{
		Log:      logger.DefaultLogger{Printer: log.Printf},
		Repeater: retry.New(retry.WithWait(5*time.Second), retry.WithAttempts(5)),
		nf:       nf,
	}
	for _, m := range mods {
		if err := m(n); err != nil {
			return nil, err
		}
	}
	if n.AuthClient == nil {
		fmt.Println("initializing auth client")
		conn, err := n.nf.af.Connect()
		if err != nil {
			return nil, err
		}
		n.AuthClient = auth.NewAuthClient(conn)
	}
	return n, nil
}
