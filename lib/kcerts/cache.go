package kcerts

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"path/filepath"
)

const (
	SSHCacheKey  = "enkitssh"
	SSHCacheFile = "ssh.json"
)

var (
	SSHAgentNoCache = errors.New("ssh agent cached entry does not exist")
)

func FetchSSHAgentFromCache(store cache.Store, mods ...SSHAgentModifier) (*SSHAgent, error) {
	sshEnkitCache, err := store.Exists(SSHCacheKey)
	if err != nil {
		return nil, fmt.Errorf("error fetching ssh agent cache: %w", err)
	}
	if sshEnkitCache == "" {
		return nil, SSHAgentNoCache
	}
	agent, err := NewSSHAgent(mods...)
	if err != nil {
		return nil, err
	}
	if err := marshal.UnmarshalFile(filepath.Join(sshEnkitCache, SSHCacheFile), &agent.SSHAgentState); err != nil {
		return nil, fmt.Errorf("error deserializing ssh agent cache: %w", err)
	}
	return agent, err
}

func WriteAgentToCache(store cache.Store, agent *SSHAgent) error {
	sshEnkitCache, _, err := store.Get(SSHCacheKey)
	if err != nil {
		return fmt.Errorf("error fetching cache: %w", err)
	}
	defer store.Rollback(sshEnkitCache)
	err = marshal.MarshalFile(filepath.Join(sshEnkitCache, SSHCacheFile), &agent.SSHAgentState)
	if err != nil {
		return fmt.Errorf("error writing to cache: %w", err)
	}
	_, err = store.Commit(sshEnkitCache)
	if err != nil {
		return err
	}
	// Agent Saved successfully, can remove kill Close
	agent.Close = func() {}
	return nil
}

// DeleteSSHCache deletes the SSH cache
func DeleteSSHCache(store cache.Store) error {
	sshEnkitCache, err := store.Exists(SSHCacheKey)
	if err != nil {
		return err
	}
	if sshEnkitCache == "" {
		return SSHAgentNoCache
	}
	return store.Purge(sshEnkitCache)
}
