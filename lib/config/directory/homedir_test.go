package directory

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenHomeDir(t *testing.T) {
	hasAcceptablePrefixes := func(path string, s ...string) bool {
		for _, ss := range s {
			if strings.HasPrefix(path, ss) {
				return true
			}
		}

		return false
	}

	os.Clearenv()
	os.Setenv("HOME", "/home/test")
	Refresh()
	dir, err := OpenHomeDir("app", "identity")
	assert.Nil(t, err)
	assert.True(t, strings.HasPrefix(dir.path, "/home/test"), "path %s", dir.path)
	os.Unsetenv("HOME")
	Refresh()
	dir, err = OpenHomeDir("app", "identity")
	assert.Nil(t, err, "%v", err)
	assert.True(t, hasAcceptablePrefixes(dir.path, "/home", "/root"), "path %s", dir.path)
}

func TestOpenDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "opendir")
	assert.Nil(t, err)

	hd, err := OpenDir(filepath.Join(dir, "test"))
	assert.Nil(t, err)

	confs, err := hd.List()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(confs))

	data, err := hd.Read("test")
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, 0, len(data))

	err = hd.Delete("test")
	assert.True(t, os.IsNotExist(err))

	quote := []byte("the burden of proof has to be placed on authority, and that it should be dismantled if that burden cannot be met")
	err = hd.Write("test", quote)
	assert.Nil(t, err)

	data, err = hd.Read("test")
	assert.Nil(t, err)
	assert.Equal(t, quote, data)

	confs, err = hd.List()
	assert.Nil(t, err)
	assert.Equal(t, []string{"test"}, confs)

	err = hd.Delete("test")
	assert.Nil(t, err)

	confs, err = hd.List()
	assert.Nil(t, err)
	assert.Equal(t, []string{}, confs)
}
