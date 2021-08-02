package mserver

import (
	"github.com/enfabrica/enkit/machinist/config"
)

type Modifier func(s *ControlPlane) error

func WithMachinistFlags(mods ...config.CommonModifier) Modifier {
	return func(s *ControlPlane) error {
		for _, mod := range mods {
			if err := mod(s); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithController(controller *Controller) Modifier {
	return func(s *ControlPlane) error {
		s.Controller = controller
		return nil
	}
}
