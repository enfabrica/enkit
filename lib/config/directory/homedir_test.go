package directory

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

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

	quote := []byte("the burden of proof has to be placed on authority, and that it should be dismantled if that burden cannot be met")
	err = hd.Write("test", quote)
	assert.Nil(t, err)

	data, err = hd.Read("test")
	assert.Nil(t, err)
	assert.Equal(t, quote, data)

	confs, err = hd.List()
	assert.Nil(t, err)
	assert.Equal(t, []string{"test"}, confs)
}
