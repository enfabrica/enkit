package mserver

import (
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"log"
)

func NewController(mods ...ControllerModifier) (*Controller, error) {
	dnsServ, err := kdns.NewDNS()
	if err != nil {
		return nil, err
	}
	en := &Controller{
		connectedNodes: map[string]*Node{},
		Log:            &logger.DefaultLogger{Printer: log.Printf},
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