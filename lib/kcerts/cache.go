package kcerts

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"path/filepath"
)

const (
	SSHCacheKey  = "enkit_ssh_cache_key"
	SSHCacheFile = "ssh.json"
)

var (
	SSHAgentNoCache      = errors.New("the cache had not existed before")
	SSHAgentCacheInvalid = errors.New("the cache is corrupted or invalid json")
)

func FetchSSHAgentFromCache(store cache.Store) (*SSHAgent, error) {
	sshEnkitCache, isFresh, err := store.Get(SSHCacheKey)
	if err != nil {
		return nil, fmt.Errorf("error fetching cache: %w", err)
	}
	defer store.Rollback(sshEnkitCache)
	agent := &SSHAgent{
		Close: func() {},
	}
	if isFresh {
		if err := marshal.UnmarshalFile(filepath.Join(sshEnkitCache, SSHCacheFile), &agent); err != nil {
			return nil, fmt.Errorf("%s: %w", SSHAgentCacheInvalid, err)
		}
		return agent, err
	}
	return nil, SSHAgentNoCache
}

func WriteAgentToCache(store cache.Store, agent *SSHAgent) error {
	sshEnkitCache, _, err := store.Get(SSHCacheKey)

	killFunc := func() {
		//TODO(adam): add logging inside ssh agent?
		if err := agent.Kill(); err != nil {
			//Right now this is swallows
		}
	}

	if err != nil {
		agent.Close = killFunc
		return fmt.Errorf("error fetching cache: %w", err)
	}
	defer store.Rollback(sshEnkitCache)
	err = marshal.MarshalFile(filepath.Join(sshEnkitCache, SSHCacheFile), agent)
	if err != nil {
		agent.Close = killFunc
		return fmt.Errorf("error writing to cache: %w", err)
	}
	_, err = store.Commit(sshEnkitCache)
	if err != nil {
		agent.Close = killFunc
	}
	return err
}

// DeleteSSHCache deletes the SSH cache
func DeleteSSHCache(store cache.Store) error {
	sshEnkitCache, _, err := store.Get(SSHCacheKey)
	if err != nil {
		return err
	}
	return store.Purge(sshEnkitCache)
}
