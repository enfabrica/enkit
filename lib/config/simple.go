package config

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/config/marshal"
)

type SimpleStore struct {
	loader     Loader
	marshaller marshal.Marshaller
}

func NewSimple(loader Loader, marshaller marshal.Marshaller) *SimpleStore {
	return &SimpleStore{loader: loader, marshaller: marshaller}
}

func (ss *SimpleStore) List() ([]string, error) {
	return ss.List()
}

func (ss *SimpleStore) Marshal(desc Descriptor, value interface{}) error {
	name, ok := desc.(string)
	if !ok {
		return fmt.Errorf("API Usage Error - SimpleStore.Marshal must be passed a string as descriptor")
	}
	data, err := ss.marshaller.Marshal(value)
	if err != nil {
		return err
	}
	return ss.loader.Write(name, data)
}

func (ss *SimpleStore) Unmarshal(name string, value interface{}) (Descriptor, error) {
	data, err := ss.loader.Read(name)
	if err != nil {
		return name, err
	}
	if len(data) <= 0 {
		return name, nil
	}
	return name, ss.marshaller.Unmarshal(data, value)
}
