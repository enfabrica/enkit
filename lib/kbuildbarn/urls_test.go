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
		hash, size, err := kbuildbarn.ByteStreamUrl(c.Url)
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
	hash, size, err := kbuildbarn.ByteStreamUrl(exampleUrl)
	assert.NoError(t, err)
	defaultGenerator := kbuildbarn.NewBuildBarnParams("buildbarn.local", hash, size)
	assert.Equal(t, "http://buildbarn.local/blobs/action/foo-bar", defaultGenerator.ActionUrl())
	assert.Equal(t, "http://buildbarn.local/blobs/command/foo-bar", defaultGenerator.CommandUrl())
	assert.Equal(t, "http://buildbarn.local/blobs/directory/foo-bar", defaultGenerator.DirectoryUrl())
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/", defaultGenerator.FileUrl())
}

func TestFileUrlGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ByteStreamUrl(exampleUrl)
	assert.NoError(t, err)
	defaultGenerator := kbuildbarn.NewBuildBarnParams("buildbarn.local", hash, size, kbuildbarn.WithFileName("mickey.mouse"))
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/mickey.mouse", defaultGenerator.FileUrl())
}
func TestByteStreamGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := kbuildbarn.ByteStreamUrl(exampleUrl)
	assert.NoError(t, err)
	defaultGenerator := kbuildbarn.NewBuildBarnParams("buildbarn.local", hash, size,
		kbuildbarn.WithScheme("bytestream"))
	assert.Equal(t, "bytestream://buildbarn.local/blobs/foo/bar", defaultGenerator.ByteStreamUrl())
}
