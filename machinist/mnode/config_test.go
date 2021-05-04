package mnode

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig(t *testing.T) {
	c := &Config{
		enrollConfigs: &enrollConfigs{},
	}
	c.HostKeyLocation = "/foo/bar.pem"
	assert.Equal(t, "/foo/bar-cert.pub", c.HostCertificate())
}
