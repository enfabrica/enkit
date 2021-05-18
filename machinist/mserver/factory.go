package mserver

import (
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
)

func NewController(mods ...ControllerModifier) (*Controller, error) {
	dnsServ, err := kdns.NewDNS()
	if err != nil {
		return nil, err
	}
	en := &Controller{
		reservedNodes: map[string]*ReservedNode{},
		connectedNodes: map[string]*Node{},
		Log:            &logger.NilLogger{},
		dnsServer: dnsServ,
	}
	for _, m := range mods {
		if err := m(en); err != nil {
			return nil, err
		}
	}
	return en, nil
}

type ControllerModifier func(*Controller) error

func DnsPort(dnsPort int) ControllerModifier {
	return func(controller *Controller) error {
		controller.dnsPort = dnsPort
		return nil
	}
}

func WithKDnsFlags(mods ...kdns.DNSModifier) ControllerModifier {
	return func(controller *Controller) error {
		for _, m :=  range mods {
			if err := m(controller.dnsServer); err != nil {
				return err
			}
		}
		return nil
	}
}
