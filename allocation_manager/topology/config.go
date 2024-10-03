package topology

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3" // package "yaml"
)

type Topology struct {
	Name string `yaml:"name"`
	// Devices ... `yaml: devices`
	Nodes map[string]Node `yaml: nodes`
	// CXLs ... `yaml: interfaces`
	GPUs map[string]GPU `yaml: GPUs`
	// Interfaces ... `yaml: interfaces`
	// NetworkConfig string `yaml:"network_config"`
	// Links ... `yaml: links`
}

type Node struct {
	Hostname string `yaml: hostname`
}

type GPU struct {
	Name  string `yaml: name`
	Vram  int    `yaml: vram`
	Busid string `yaml: busid` // todo remove
}

func ParseYaml(configBytes []byte) (*Topology, error) {
	var topology Topology
	// TODO: need "global" passthrough for undefined things I haven't explicitly created data structure to capture.
	err := yaml.Unmarshal(configBytes, &topology)
	return &topology, err
}

func LoadYaml(filename string) (*Topology, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseYaml(bytes)
}

func SaveYaml(filename string, topology *Topology) error {
	bytes, err := yaml.Marshal(topology)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0666)
}

func WriteYaml(fh *os.File, topology *Topology) error {
	bytes, err := yaml.Marshal(topology)
	if err != nil {
		return err
	}
	num, err := fh.Write(bytes)
	if err != nil {
		return err
	}
	if num != len(bytes) {
		return fmt.Errorf("Wrote %d bytes, want %d", num, len(bytes))
	}
	return nil
}
