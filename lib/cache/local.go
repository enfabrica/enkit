package cache

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/kirsle/configdir"
)

// Local implements a local file system based cache.
type Local struct {
	Root string
}

// NewLocal will return a Local cache pointing to the OS specific directory where files are cached.
// label is the name of the subdirectory, it should not be empty.
//
// Example:
//   l := NewLocal("gnome")
//
// Will return /home/username/.cache/gnome as the location for the cache.
func NewLocal(label string) *Local {
	return &Local{Root: configdir.LocalCache(label)}
}

func (l *Local) Register(flags kflags.FlagSet, prefix string) *Local {
	flags.StringVar(&l.Root, prefix+"cache-dir", l.Root, "Directory where to cache files")
	return l
}

func hash(key string) []byte {
	h := sha256.New()
	h.Write([]byte(key))
	return h.Sum(nil)
}

// Get returns a directory name where to store data corresponding to key.
//
// key is a unique identifier for the data to be stored (or retrieved) from
// cache. It can be, for example, the URL for a large download, or the id
// of an object in a database.
//
// Get will return three parameters:
//    - the path to a directory, where to store or read the data corresponding
//      to this key.
//    - a boolean indicating if there was already a directory associated with
//      this key (true), or if a new directory was created from scratch (false).
//      When true, you should expect data previously stored to be ready and available
//      in the directory, but see below.
//    - an error, normally nil, indicating any error that happened while
//      trying to create this cached object.
//
// For a cache to be effective in an environment supporting parallel builds:
//    - only complete and usable artifacts should be stored in cache.
//    - data in cache should never be modified.
//
// To ensure both:
//    1) When using the cache, call Get()
//    2) If the data is there already, just use it.
//    3) If not there, download it, generate it, ... and store it directly in
//       the directory returned by Get().
//    4) Once the download, build, ... process is completed successfully, call
//       Commit(), described below. In case of error, call Purge().
//
// Between 3 and 4, the Local implementation guarantees that no other job will
// see the partial results being built or downloaded, the key will become available
// from cache (and results stored) only after Commit() is called.
//
// If you need to make changes to values in Local(), use Clone().
func (c *Local) Get(key string) (string, bool, error) {
	sum := hash(key)
	dirPrefix := filepath.Join(c.Root, fmt.Sprintf("%x", sum[0:1]))
	dirEnd := fmt.Sprintf("%x", sum[1:len(sum)-1])
	dirFull := filepath.Join(dirPrefix, dirEnd)
	if PathIsDir(dirFull) {
		return dirFull, true, nil
	}
	err := os.MkdirAll(dirPrefix, 0750)
	if err != nil {
		return "", false, err
	}
	dirFull, err = ioutil.TempDir(dirPrefix, dirEnd+".tmp")
	if err != nil {
		return "", false, err
	}
	return dirFull, false, nil
}

// Returns true if a cache entry by the specified key exists already.
//
// Note that this is no guarantee that the cache key will exist by the
// time you use it. If you need a cache key in a concurrent environment,
// you should always use Get(), Commit(), and Purge().
func (c *Local) Exists(key string) (string, error) {
	sum := hash(key)
	dirPrefix := filepath.Join(c.Root, fmt.Sprintf("%x", sum[0:1]))
	dirEnd := fmt.Sprintf("%x", sum[1:len(sum)-1])
	dirFull := filepath.Join(dirPrefix, dirEnd)
	if PathIsDir(dirFull) {
		return dirFull, nil
	}
	return "", nil
}

// Commits a cache location so others can start retrieving it.
//
// When Get() encounters a key that was not in cache already, it returns a temporary
// location where to store the data. This location is not available for others to use
// until it is committed with a Commit() call.
//
// It is recommended that Commit() is only called once the data to be cached has
// successfully been generated and verified - once it can be treated as a fully complete
// read only resource.
//
// If your code, however, can deal with inconsistent/incomplete data in cache, or
// with concurrent write access to data in a common shared directory, you can
// call Commit() immediately after Get().
//
// The `location` string must be a value returned by Get().
func (c *Local) Commit(location string) (string, error) {
	idx := strings.LastIndex(location, ".tmp")
	// Defense in depth - ensure that the supplied path to commit is valid.
	if c.Root != "." && !strings.HasPrefix(location, c.Root) {
		panic(fmt.Sprintf("Tried to commit '%s' - which is not a temporary cache path", location))
	}
	// Tolerate Committing an already committed directory. This simplifies user's code.
	if idx < 0 {
		return location, nil
	}
	// If we are here, it's because Get() returned a not found (the key did not exist),
	// a temporary location was returned, and now we are turning it into our final destination.
	// If we fail turning it into a final destination, it means we raced with another thread
	// also calling Get() and Commit() in parallel.
	// We should not fail here. Instead, we let the other thread win. It was faster.
	if err := os.Rename(location, location[:idx]); err != nil && !os.IsExist(err) {
		return location[:idx], err
	}
	return location[:idx], nil
}

// Purge removes and frees up the resources associated with a location generated by Get.
//
// Purge is used in two different cases:
//   1) To remove an existing entry from cache. Invoke Get for the key, pass the location
//      to Purge() to eliminate it from cache.
//   2) To remove any resource associated with a failed attempt at filling a cache entry.
//      Let's say Get() returned a new location. Your code is preparing data to be stored
//      in cache, but the build fails. To free up resources in this cache entry, call
//      Purge(location).
//
// Note that once a resource is Commit()ted, the location changes. So if Purge() is invoked
// on a resource just committed, it will just fail without doing anything.
// It is thus recommended that a defer c.Purge(location) is set up every time a new cache
// entry is being filled, like with:
//
//     location, ready, err := cache.Get("downloaded-libc")
//     if !ready {
//         defer cache.Purge(location)
//         // retry downloading file.
//         if err != nil {
//              return err // cache.Purge() will be invoked, and clean up.
//         }
//         cache.Commit(location)
//         // cache.Purge, when invoked by the defer, is now a noop.
//     }
func (c *Local) Purge(location string) error {
	// Defense in depth - ensure that the supplied path to remove is valid.
	location = filepath.Clean(location)
	if c.Root != "." && !strings.HasPrefix(location, c.Root) {
		panic(fmt.Sprintf("Tried to purge '%s' - outside the root of the cache", location))
	}
	// Ensure no partial file is read from the cache while deletion is in progress.
	tempname := fmt.Sprintf("%s-%016x", location, rand.Uint64())
	os.Rename(location, tempname)
	return os.RemoveAll(tempname)
}

// Rollback purges the directory if it has not been committed.
func (c *Local) Rollback(location string) error {
	idx := strings.LastIndex(location, ".tmp")
	if idx < 0 {
		return nil
	}
	return c.Purge(location)
}

// PathIsDir returns true if the specified path is a directory.
func PathIsDir(directory string) bool {
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return false
	}
	return true
}
