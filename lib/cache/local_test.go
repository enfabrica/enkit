package cache

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCache(t *testing.T) {
	cacheRoot, err := ioutil.TempDir("", "cache")
	if err != nil {
		t.Errorf("cannot create temporary test directory - %v", err)
	}
	defer os.RemoveAll(cacheRoot)

	cache := &Local{Root: cacheRoot}

	// Get a key that does not exist.
	location, found, err := cache.Get("test-key")
	if err != nil {
		t.Errorf("getting a key failed - %v", err)
	}
	if !strings.HasPrefix(location, cacheRoot) {
		t.Errorf("invalid location for cache key")
	}
	if found == true {
		t.Errorf("found a key that did not exist??")
	}
	// Write some data in the cache, without committing.
	if err := ioutil.WriteFile(filepath.Join(location, "test.txt"), []byte("this is a test file"), 0600); err != nil {
		t.Errorf("Could not write a file in %s", location)
	}

	exists, err := cache.Exists("test-key")
	if exists != "" {
		t.Errorf("key unexpectedly found? it has not been committed yet!!")
	}
	if err != nil {
		t.Errorf("cache.Exists failed? %s", err)
	}

	// Key still does not exist, it has not been committed.
	location2, found, _ := cache.Get("test-key")
	if found == true {
		t.Errorf("found a key that was not committed??")
	}
	// And despite the fact that both keys are the same, the location is different.
	if location == location2 {
		t.Errorf("two uncommitted locations should be different")
	}
	// Commit both locations, first one should succeed.
	if _, err := cache.Commit(location); err != nil {
		t.Errorf("commit of first location should have succeeded - %v", err)
	}
	dest, err := cache.Commit(location2)
	if err != nil {
		t.Errorf("commit of second location should have worked - %v", err)
	}
	if _, err := ioutil.ReadFile(filepath.Join(dest, "test.txt")); err != nil {
		t.Errorf("what happened to the committed file?? %v", err)
	}

	exists, err = cache.Exists("test-key")
	if exists == "" {
		t.Errorf("key not found? after commit?")
	}
	if err != nil {
		t.Errorf("cache.Exists failed? %s", err)
	}

	// Purge both locations. Purge only return errors if something goes wrong
	// in deleting files. If nothing is left to delete, returns success.
	if err := cache.Purge(location2); err != nil {
		t.Errorf("Purging uncommitted location should have succeeded - %v", err)
	}
	if err := cache.Purge(location); err != nil {
		t.Errorf("Purging committed location should have succeeded - %v", err)
	}

	// Purge the real location.
	if err := cache.Purge(dest); err != nil {
		t.Errorf("Purging the final location - eliminating a key - should have succeeded - %v", err)
	}

	exists, err = cache.Exists("test-key")
	if exists != "" {
		t.Errorf("key still found? after purge?")
	}
	if err != nil {
		t.Errorf("cache.Exists failed? %s", err)
	}
}

// Tries to Purge a directory that was not created with Cache.Get()
func TestCachePurgePanic(t *testing.T) {
	cacheRoot, err := ioutil.TempDir("", "cache")
	if err != nil {
		t.Errorf("cannot create temporary test directory - %v", err)
	}
	defer os.RemoveAll(cacheRoot)
	cache := &Local{Root: cacheRoot}

	toRemove, err := ioutil.TempDir("", "a-dir-to-remove-outside-cache")
	if err != nil {
		t.Errorf("cannot create directory to remove - %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	cache.Purge(toRemove)
}

// Tries to commit a directory that was not created with Cache.Get()
func TestCacheCommitPanic(t *testing.T) {
	cacheRoot, err := ioutil.TempDir("", "cache")
	if err != nil {
		t.Errorf("cannot create temporary test directory - %v", err)
	}
	defer os.RemoveAll(cacheRoot)
	cache := &Local{Root: cacheRoot}

	toCommit, err := ioutil.TempDir("", "a-dir-to-remove-outside-cache")
	if err != nil {
		t.Errorf("cannot create directory to remove - %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	cache.Commit(toCommit)
}
