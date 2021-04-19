package mnode_test

import (
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig(t *testing.T) {
	c := mnode.Config{
		HostKeyLocation: "/foo/bar.pem",
	}
	assert.Equal(t, "/foo/bar-cert.pub", c.HostCertificate())
}
