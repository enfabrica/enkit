package state

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"os"
	"sync"
)



type MachineController struct {
	sync.RWMutex
	Machines []*machinist.StaticMachine
}

// AddMachine adds a machine to the parsed in state. If a machine exists with the same name, it returns an error.
func AddMachine(mc *MachineController, m *machinist.StaticMachine) error {
	mc.Lock()
	defer mc.Unlock()
	for _, mm := range mc.Machines {
		if mm.Name == m.Name {
			return fmt.Errorf("machinist: named machine %s already exists", m.Name)
		}
	}
	mc.Machines = append(mc.Machines, m)
	return nil
}

// GetMachine fetches a machine from the state. If no machine exists with the name, it returns nil.
func GetMachine(mc *MachineController, name string) *machinist.StaticMachine {
	mc.RLock()
	defer mc.RUnlock()
	for _, mm := range mc.Machines {
		if mm.Name == name {
			return mm
		}
	}
	return nil
}

// ReadInController will attempt to read in the filepath provided and deserialize it into the machine controller.
// Fails if the file exists and cannot deserialize. If the file does not exist, it will create tje file and return a fresh state.
func ReadInController(filepath string) (*MachineController, error) {
	m := &MachineController{}
	err := marshal.UnmarshalFile(filepath, m)
	if os.IsNotExist(err) {
		err = WriteController(&MachineController{}, filepath)
		return &MachineController{}, err
	}
	return m, err
}

func WriteController(mc *MachineController, path string) error {
	mc.RLock()
	defer mc.RUnlock()
	return marshal.MarshalFile(path, mc)
}
