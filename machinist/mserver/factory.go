package mserver

import (
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/state"
	"log"
	"time"
)

func NewController(mods ...ControllerModifier) (*Controller, error) {
	en := &Controller{
		State:         &state.MachineController{},
		stateWriteTTL: time.Second * 30,
		Log:           &logger.DefaultLogger{Printer: log.Printf},
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
		if controller.dnsServer == nil {
			dnsServ, err := kdns.NewDNS(mods...)
			if err != nil {
				return err
			}
			controller.dnsServer = dnsServ
		}
		return nil
	}
}

func WithStateFile(filepath string) ControllerModifier {
	return func(controller *Controller) error {
		s, err := state.ReadInController(filepath)
		if err != nil {
			controller.Log.Warnf("Unable to read in state, err: %v", err)
			return nil
		}
		controller.State = s
		controller.stateFile = filepath
		return nil
	}
}

func WithStateWriteDuration(duration string) ControllerModifier {
	return func(controller *Controller) error {
		d, err := time.ParseDuration(duration)
		if err != nil {
			return err
		}
		controller.stateWriteTTL = d
		return nil
	}
}
