package kbuildbarn_test

import (
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	"github.com/stretchr/testify/assert"
	"testing"
)

type BytStreamResult struct {
	ShouldFail   bool
	Url          string
	ExpectedHash string
	ExpectedSize string
}

var TestByteStreamUrlTable = []BytStreamResult{
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/blobs/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
		ShouldFail:   false,
		ExpectedHash: "a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a",
		ExpectedSize: "465",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar",
		ShouldFail:   false,
		ExpectedHash: "foo",
		ExpectedSize: "bar",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000////",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
}

func TestByteStreamUrl(t *testing.T) {
	for _, c := range TestByteStreamUrlTable {
		hash, size, err := kbuildbarn.ParseByteStreamUrl(c.Url)
		if c.ShouldFail {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, c.ExpectedHash, hash)
			assert.Equal(t, c.ExpectedSize, size)
		}
	}
}

func TestDefaultUrlGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ParseByteStreamUrl(exampleUrl)
	assert.NoError(t, err)
	baseName := "buildbarn.local"
	assert.Equal(t, "http://buildbarn.local/blobs/action/foo-bar", kbuildbarn.Url(baseName, hash, size, kbuildbarn.WithActionUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/command/foo-bar", kbuildbarn.Url(baseName, hash, size, kbuildbarn.WithCommandUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/directory/foo-bar", kbuildbarn.Url(baseName, hash, size, kbuildbarn.WithDirectoryUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/", kbuildbarn.Url(baseName, hash, size, kbuildbarn.WithFileName("")))
}

func TestFileUrlGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ParseByteStreamUrl(exampleUrl)
	basename := "buildbarn.local"
	assert.NoError(t, err)
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/mickey.mouse", kbuildbarn.Url(basename, hash, size, kbuildbarn.WithFileName("mickey.mouse")))
}
func TestByteStreamGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ParseByteStreamUrl(exampleUrl)
	basename := "buildbarn.local"
	assert.NoError(t, err)
	assert.Equal(t, "bytestream://buildbarn.local/blobs/foo/bar", kbuildbarn.Url(basename, hash, size, kbuildbarn.WithByteStreamTemplate()))
}

func TestFileGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ParseByteStreamUrl(exampleUrl)
	basename := "/root"
	assert.NoError(t, err)
	assert.Equal(t, "/root/blobs/file/foo-bar/foo.go", kbuildbarn.File(basename, hash, size, kbuildbarn.WithFileName("foo.go")))
}
