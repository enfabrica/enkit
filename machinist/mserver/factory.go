package mserver

func NewController() (*Controller, error) {
	en := &Controller{
		connectedNodes: map[string]*Node{},
	}
	return en, nil
}

type ControllerModifier func(*Controller) error
