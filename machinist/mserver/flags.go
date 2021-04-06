package mserver

import "github.com/enfabrica/enkit/machinist"

type Modifier func(s *server) error

func WithMachinistFlags(mods ...machinist.Modifier) Modifier {
	return func(s *server) error {
		for _, mod := range mods {
			if err := mod(s); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithController(controller *Controller) Modifier {
	return func(s *server) error {
		s.Controller = controller
		return nil
	}
}
