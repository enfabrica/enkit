package topology

import (
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
	Busid string `yaml: busid`
}

func ParseYaml(configBytes []byte) (*Topology, error) {
	var topology Topology
	err := yaml.Unmarshal(configBytes, &topology)
	return &topology, err
}
