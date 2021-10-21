package testutil

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/psanford/memfs"
)

type FS struct {
	fs *memfs.FS
}

func (f *FS) Open(name string) (fs.File, error) {
	return f.fs.Open(name)
}

func NewFS(t *testing.T, files map[string][]byte) *FS {
	t.Helper()
	rootFS := memfs.New()
	for filename, contents := range files {
		dir, _ := filepath.Split(filename)
		if err := rootFS.MkdirAll(strings.TrimSuffix(dir, "/"), 0777); err != nil {
			t.Fatal(err)
			return nil
		}
		if err := rootFS.WriteFile(filename, contents, 0755); err != nil {
			t.Fatal(err)
			return nil
		}
	}
	return &FS{fs: rootFS}
}
