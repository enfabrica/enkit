package topology

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYamlEmpty(t *testing.T) {
	_, err := ParseYaml([]byte(""))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
}

func TestParseYamlName(t *testing.T) {
	topo, err := ParseYaml([]byte("name: alfa\n"))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
	assert.Equalf(t, "alfa", topo.Name, "Name failed to parse: %v", err)
}

func TestParseYamlNode(t *testing.T) {
	topo, err := ParseYaml([]byte("nodes:\n node01:\n  hostname: \"bravo\"\n"))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
	assert.Equalf(t, 1, len(topo.Nodes), "Nodes: missing: %v", topo)
	if len(topo.Nodes) == 1 {
		n := topo.Nodes["node01"]
		assert.Equalf(t, "bravo", n.Hostname, "hostname b incorrect")
	}
}

func TestParseYamlNodes(t *testing.T) {
	topo, err := ParseYaml([]byte("nodes:\n node01:\n  hostname: \"charlie\"\n node02:\n  hostname: \"delta\""))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
	assert.Equalf(t, 2, len(topo.Nodes), "Nodes: missing: %v", topo)
	if len(topo.Nodes) == 2 {
		n := topo.Nodes["node01"]
		assert.Equalf(t, "charlie", n.Hostname, "hostname b incorrect")
		n = topo.Nodes["node02"]
		assert.Equalf(t, "delta", n.Hostname, "hostname c incorrect")
	}
}

func TestParseYamlGPU(t *testing.T) {
	// also testing: name; two words without quotes
	// busid requires quotes
	topo, err := ParseYaml([]byte("gpus:\n GPU1:\n  name: echo foxtrot\n  busid: \"0:C5:00.0\"\n  vram: 16\n"))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
	assert.Equalf(t, 1, len(topo.GPUs), "GPUs: missing: %v", topo)
	if len(topo.GPUs) == 1 {
		g := topo.GPUs["GPU1"]
		assert.Equalf(t, "echo foxtrot", g.Name, "Error name")
		assert.Equalf(t, "0:C5:00.0", g.Busid, "Error busid")
		assert.Equalf(t, 16, g.Vram, "Error vram")
	}
}

func TestParseYamlGPUs(t *testing.T) {
	topo, err := ParseYaml([]byte("gpus:\n GPU1:\n  name: echo foxtrot\n GPU2:\n  name: golf\n"))
	assert.Equalf(t, nil, err, "ParseYaml returned error: %v", err)
	assert.Equalf(t, 2, len(topo.GPUs), "GPUs: missing: %v", topo)
	if len(topo.GPUs) == 2 {
		g := topo.GPUs["GPU1"]
		assert.Equalf(t, "echo foxtrot", g.Name, "Error name")
		g = topo.GPUs["GPU2"]
		assert.Equalf(t, "golf", g.Name, "Error name")
	}
}
