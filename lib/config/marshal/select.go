package marshal

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// Use marshal.Toml to encode/decode from Toml format.
var Toml = &TomlEncoder{}

// Use marshal.Yaml to encode/decode from Yaml format.
var Yaml = &YamlEncoder{}

// Use marshal.Json to encode/decode from Json format.
var Json = &JsonEncoder{}

// Set of known encoders/decoders, in preference order.
var Known = []FileMarshaller{
	Toml, Json, Yaml,
}

type FileMarshallers []FileMarshaller

// ByExtension returns the best FileMarshaller based on the extension of the path provided.
func (fm FileMarshallers) ByExtension(path string) FileMarshaller {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		return nil
	}

	for _, candidate := range fm {
		if candidate.Extension() == ext {
			return candidate
		}
	}
	return nil
}

func (fm FileMarshallers) Marshal(path string, value interface{}) ([]byte, error) {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return nil, fmt.Errorf("could not determine format from path %s - unknown extension?", path)
	}
	return marshaller.Marshal(value)
}

func (fm FileMarshallers) MarshalDefault(path string, def Marshaller, value interface{}) ([]byte, error) {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return def.Marshal(value)
	}
	return marshaller.Marshal(value)
}

func (fm FileMarshallers) Unmarshal(path string, data []byte, value interface{}) error {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return fmt.Errorf("could not determine format from path %s - unknown extension?", path)
	}
	return marshaller.Unmarshal(data, value)
}

func (fm FileMarshallers) UnmarshalDefault(path string, data []byte, def Marshaller, value interface{}) error {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return def.Unmarshal(data, value)
	}
	return marshaller.Unmarshal(data, value)
}

func Marshal(path string, value interface{}) ([]byte, error) {
	return FileMarshallers(Known).Marshal(path, value)
}

func Unmarshal(path string, data []byte, value interface{}) error {
	return FileMarshallers(Known).Unmarshal(path, data, value)
}

func MarshalFile(path string, value interface{}) error {
	data, err := Marshal(path, value)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0660)
}
func UnmarshalFile(path string, value interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return Unmarshal(path, data, value)
}

func MarshalDefault(path string, def Marshaller, value interface{}) ([]byte, error) {
	return FileMarshallers(Known).MarshalDefault(path, def, value)
}

func UnmarshalDefault(path string, data []byte, def Marshaller, value interface{}) error {
	return FileMarshallers(Known).UnmarshalDefault(path, data, def, value)
}
