package karchive

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestMkdir(t *testing.T) {
	td, err := ioutil.TempDir("", "test-mkdir")
	assert.Nil(t, err, "%v", err)

	created, err := MkdirAll(td+"/foo/bar/.././buz/", 0741)
	assert.Nil(t, err)
	assert.Equal(t, []string{td + "/foo/buz", td + "/foo"}, created)

	for _, dir := range created {
		stat, err := os.Stat(dir)
		assert.Nil(t, err)
		assert.Equal(t, uint32(0741), uint32(stat.Mode().Perm()))
	}

	created, err = MkdirAll(td+"/foo/bar/.././buz/", 0743)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, created)

	created, err = MkdirAll("/", 0743)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, created)
}
