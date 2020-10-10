package kflags

import (
	"github.com/enfabrica/enkit/lib/logger"
	"path/filepath"
	"strings"
)

type asset struct {
	// Original name of the asset.
	name string
	// Byte content of the asset.
	data []byte
}

type AssetAugmenter struct {
	log   logger.Logger
	index map[string]asset
	forns string
}

// NewAssetAugmenter creates a new AssetAugmenter.
//
// An asset-resolver looks up configuration flags in a built in dict where the key is
// the name of a file, and the value is what should be passed to the flag.
// Extensions of the key are ignored.
//
// For example, let's say you have a dict containing:
//
//     "/etc/astore/astore-server.flag.txt": "127.0.0.1"
//
// Now let's say you have a binary that takes a --astore-server or -astore-server flag.
//
// When invoked, the returned AssetAugmenter will set the default value of --astore-server to 127.0.0.1.
//
// This is extremely powerful when combined to a library to embed files at build time,
// like the go_embed_data target of bazel.
func NewAssetAugmenter(log logger.Logger, forns string, assets map[string][]byte) *AssetAugmenter {
	index := map[string]asset{}
	for name, data := range assets {
		original := name
		for {
			index[name] = asset{
				name: original,
				data: data,
			}
			ext := filepath.Ext(name)
			if ext == "" {
				break
			}
			name = strings.TrimSuffix(name, ext)
		}
	}

	return &AssetAugmenter{
		log:   log,
		index: index,
		forns: forns,
	}
}

// Visit implements the Visit interface of Augmenter.
func (ar *AssetAugmenter) Visit(reqns string, fl Flag) (bool, error) {
	if reqns != ar.forns {
		ar.log.Debugf("%s flag %s: no asset assigned - namespace %s != %s", ar.forns, fl.Name(), reqns, ar.forns)
		return false, nil
	}

	asset, found := ar.index[fl.Name()]
	if !found {
		ar.log.Debugf("%s flag %s: not found among assets - not in index", ar.forns, fl.Name())
		return false, nil
	}

	ar.log.Infof("%s flag %s: set from static assets (%d bytes)", ar.forns, fl.Name(), len(asset.data))
	return true, fl.SetContent(asset.name, asset.data)
}

// Done implements the Done interface of Augmenter.
func (ar *AssetAugmenter) Done() error {
	return nil
}
