package exec

import (
	"embed"
	"path/filepath"
)

const (
	clientExecutableName = "bb_clientd"
)

//go:embed templates/*
var templates embed.FS

// ClientOptions contains all the configuration needed to start bb_clientd.
type Client struct {
	InstanceName  string
	MountDir      string
	ScratchSuffix string
}

// NewClient returns options with sane defaults based on the specified
// outputBase.
func NewClient(instanceName string, outputBase string, scratchSuffix string) *Client {
	return &Client{
		InstanceName:  instanceName,
		MountDir:      outputBase,
		ScratchSuffix: scratchSuffix,
	}
}

func (o *Client) ScratchDir() string {
	return filepath.Join(o.MountDir, "/scratch", o.ScratchSuffix)
}
