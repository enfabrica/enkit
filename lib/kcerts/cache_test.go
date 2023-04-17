package kcerts

import (
	"os"

	"github.com/enfabrica/enkit/lib/cache"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "en")
	assert.NoError(t, err)

	store := &cache.Local{
		Root: tmpDir,
	}
	agent, err := NewSSHAgent()
	assert.NoError(t, err)

	// No agent data saved - this should fail!
	assert.Error(t, agent.LoadFromCache(store))

	// ... deleting the non existing data should also fail.
	assert.Error(t, DeleteSSHCache(store))

	agent, err = NewSSHAgent()
	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// Write agent data, and read it back. Should succeed.
	agent.PID = 1789
	agent.Socket = "/tmp/non-existing/test"
	assert.Nil(t, WriteAgentToCache(store, agent))
	readback, err := NewSSHAgent()
	assert.NoError(t, err)
	assert.NotNil(t, readback)
	assert.NoError(t, readback.LoadFromCache(store))
	assert.Equal(t, 1789, readback.PID)
	assert.Equal(t, "/tmp/non-existing/test", readback.Socket)

	// Do it again, just to make sure the file is still writable.
	agent.PID = 9993
	agent.Socket = "/tmp/non-existing/again"
	assert.Nil(t, WriteAgentToCache(store, agent))
	readback, err = NewSSHAgent()
	assert.NoError(t, err)
	assert.NotNil(t, readback)
	assert.NoError(t, readback.LoadFromCache(store))
	assert.NoError(t, err)
	assert.Equal(t, 9993, readback.PID)
	assert.Equal(t, "/tmp/non-existing/again", readback.Socket)

	// Deleting the cache should succeed.
	assert.NoError(t, DeleteSSHCache(store))
	agent, err = NewSSHAgent()
	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Error(t, agent.LoadFromCache(store))
}
