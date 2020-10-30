package kconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/karchive"
	"github.com/enfabrica/enkit/lib/khttp/kcache"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"

	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type CommandRetriever struct {
	cache   cache.Store
	mods    []protocol.Modifier
	log     logger.Logger
	retrier *retry.Options
}

func NewCommandRetriever(log logger.Logger, cache cache.Store, retrier *retry.Options, mods ...protocol.Modifier) *CommandRetriever {
	return &CommandRetriever{
		cache:   cache,
		mods:    mods,
		log:     log,
		retrier: retrier,
	}
}

func (cr *CommandRetriever) PrepareHash(url, hash string) (string, error) {
	hash = strings.TrimSpace(hash)

	unpack, err := cr.cache.Exists(hash)
	if err != nil {
		return "", fmt.Errorf("problem accessing cached entry for hash %s of %s - %w", hash, url, err)
	}
	if unpack != "" {
		return unpack, nil
	}

	if err := cr.retrier.Run(func() error {
		return protocol.Get(url, protocol.Reader(func(httpr io.Reader) error {
			var found bool
			var err error	

			unpack, found, err = cr.cache.Get(hash)
			if found {
				return nil
			}
			defer cr.cache.Rollback(unpack)

			h := sha256.New()
			r := io.TeeReader(httpr, h)
			err = karchive.Untarz(url, r, unpack, karchive.WithFileUmask(0222))
			if err != nil {
				return fmt.Errorf("error decompressing %s: %w", url, err)
			}

			computed := hex.EncodeToString(h.Sum(nil))
			if hash != computed {
				return fmt.Errorf("computed sha256 for %s is %s - required is %s - REJECTED", url, computed, hash)
			}

			unpack, err = cr.cache.Commit(unpack)
			return err
		}), cr.mods...)
	}); err != nil {
		return "", err
	}

	return unpack, nil
}

func (cr *CommandRetriever) PrepareURL(url string) (string, error) {
	var unpack string
	if err := cr.retrier.Run(func() error {
		mods := protocol.Modifiers{}
		mods = append(mods, kcache.WithCache(cr.cache, kcache.WithLogger(cr.log)))
                mods = append(mods, cr.mods...)

		return protocol.Get(url, protocol.Reader(func(r io.Reader) error {
			cf, converted := r.(*kcache.CachedFile)
			if !converted {
				return retry.Fatal(fmt.Errorf("internal error: expected a CachedFile, but conversion failed. Got %#v", r))
			}

			tmp, found, err := cr.cache.Get(cf.Path)
			if err != nil {
				return retry.Fatal(fmt.Errorf("problem accessing cached entry %s for %s - %w", cf.Path, url, err))
			}
			if found {
				unpack = tmp
				return nil
			}
			defer cr.cache.Rollback(tmp)
			if err := karchive.Untarz(url, cf, unpack, karchive.WithFileUmask(0227)); err != nil {
				return err
			}
			unpack, err = cr.cache.Commit(tmp)
			return err
		}), mods...)
	}); err != nil {
		return "", err
	}

	return unpack, nil
}

func (cr *CommandRetriever) Prepare(url, hash string) (string, error) {
	if hash != "" {
		return cr.PrepareHash(url, hash)
	}
	return cr.PrepareURL(url)
}

func (cr *CommandRetriever) Retrieve(url, hash string) (string, *Manifest, error) {
	dir, err := cr.Prepare(url, hash)
	if err != nil {
		return "", nil, err
	}

	var manifest Manifest
	if _, err := marshal.UnmarshalFilePrefix(filepath.Join(dir, "manifest"), &manifest); err != nil {
		return "", nil, fmt.Errorf("could not find manifest file: %w", err)
	}
	return dir, &manifest, nil
}
