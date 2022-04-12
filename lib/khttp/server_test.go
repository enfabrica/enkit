package khttp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddDefaultPort(t *testing.T) {
	address, err := addDefaultPort("", 0)
	assert.Error(t, err)

	address, err = addDefaultPort("", 65536)
	assert.Error(t, err)

	address, err = addDefaultPort("", 80)
	assert.NoError(t, err)
	assert.Equal(t, ":80", address)

	address, err = addDefaultPort("127.0.0.1", 80)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:80", address)

	address, err = addDefaultPort("[::1]", 80)
	assert.NoError(t, err)
	assert.Equal(t, "[::1]:80", address)

	address, err = addDefaultPort("127.0.0.1:1234", 80)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:1234", address)

	address, err = addDefaultPort("[::1]:1234", 80)
	assert.NoError(t, err)
	assert.Equal(t, "[::1]:1234", address)
}
