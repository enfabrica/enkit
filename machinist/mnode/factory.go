package mnode

import "github.com/enfabrica/enkit/machinist"

func New(mods ...NodeModifier) (*Node, error) {
	n := &Node{
		SharedFlags: &machinist.SharedFlags{},
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
