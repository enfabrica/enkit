package mserver

func NewController() (*Controller, error) {
	en := &Controller{
		ConnectedNodes: make(map[string][]string),
	}
	return en, nil
}

type ControllerModifier func(*Controller) error
