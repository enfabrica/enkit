package kassets

import (
	"embed"
	"fmt"
	"io/fs"
)

// FS wraps the required interfaces for reading embedded assets into a named
// type, since the `fs` package doesn't seem to have a pre-made interface for
// this already.
type FS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

// EmbedSubdir returns an FS implementation for a subdirectory of an embed.FS,
// which is useful for stripping the top-level directory.
func EmbedSubdir(f embed.FS, subdir string) (FS, error) {
	subdirFS, err := fs.Sub(f, subdir)
	if err != nil {
		return nil, fmt.Errorf("can't create FS for subdir %q: %w", err)
	}
	if casted, ok := subdirFS.(FS); !ok {
		return nil, fmt.Errorf("can't cast fs.FS impl %T to AssetFS", subdirFS)
	} else {
		return casted, nil
	}
}

// MustEmbedSubdir is a variation of EmbedSubdir that panics on error, which is
// useful for concise initialization.
func MustEmbedSubdir(f embed.FS, subdir string) FS {
	assetFS, err := EmbedSubdir(f, subdir)
	if err != nil {
		panic(err)
	}
	return assetFS
}

// MapFromFS returns a map of (filename -> contents) for each entry in the
// AssetFS. This helps adapt code that expected embedded data to be supplied via
// such maps with the new way of embedding assets (via embed.FS).
func MapFromFS(f FS) (map[string][]byte, error) {
	assetMap := map[string][]byte{}

	entries, err := f.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("unable to list assets from AssetFS: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := f.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read asset %q from AssetFS: %w", entry.Name(), err)
		}
		assetMap[entry.Name()] = data
	}

	return assetMap, nil
}

// MustMapFromFS is a variation of EmbedSubdir that panics on error, which is
// useful for concise initialization.
func MustMapFromFS(f FS) map[string][]byte {
	m, err := MapFromFS(f)
	if err != nil {
		panic(err)
	}
	return m
}
