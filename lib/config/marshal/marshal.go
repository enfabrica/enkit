package marshal

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

type PrettyJsonEncoder struct{}

func (j *PrettyJsonEncoder) Marshal(value interface{}) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
func (j *PrettyJsonEncoder) Unmarshal(data []byte, value interface{}) error {
	return json.Unmarshal(data, value)
}
func (j *PrettyJsonEncoder) Extension() string {
	return "json"
}

type JsonEncoder struct{}

func (j *JsonEncoder) Marshal(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}
func (j *JsonEncoder) Unmarshal(data []byte, value interface{}) error {
	return json.Unmarshal(data, value)
}
func (j *JsonEncoder) Extension() string {
	return "json"
}

type TomlEncoder struct{}

func (j *TomlEncoder) Marshal(value interface{}) ([]byte, error) {
	return toml.Marshal(value)
}
func (j *TomlEncoder) Unmarshal(data []byte, value interface{}) error {
	return toml.Unmarshal(data, value)
}
func (j *TomlEncoder) Extension() string {
	return "toml"
}

type YamlEncoder struct{}

func (j *YamlEncoder) Marshal(value interface{}) ([]byte, error) {
	return yaml.Marshal(value)
}
func (j *YamlEncoder) Unmarshal(data []byte, value interface{}) error {
	return yaml.Unmarshal(data, value)
}
func (j *YamlEncoder) Extension() string {
	return "yaml"
}

type GobEncoder struct{}

func (g *GobEncoder) Marshal(value interface{}) ([]byte, error) {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func (j *GobEncoder) Unmarshal(data []byte, value interface{}) error {
	enc := gob.NewDecoder(bytes.NewReader(data))
	if err := enc.Decode(value); err != nil {
		return err
	}
	return nil
}
func (j *GobEncoder) Extension() string {
	return "gob"
}
