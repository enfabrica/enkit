package mnode

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"log"
	"time"
)

func New(mods ...NodeModifier) (*Node, error) {
	n := &Node{
		Log:      logger.DefaultLogger{Printer: log.Printf},
		Repeater: retry.New(retry.WithWait(5*time.Second), retry.WithAttempts(5)),
	}
	for _, m := range mods {
		if err := m(n); err != nil {
			return nil, err
		}
	}
	if err := n.Init(); err != nil {
		return nil, err
	}
	return n, nil
}
