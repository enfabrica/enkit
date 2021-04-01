package mserver

func NewController() (*Controller, error) {
	en := &Controller{
		connectedNodes: make(map[string]*Node),
	}
	return en, nil
}

type ControllerModifier func(*Controller) error
