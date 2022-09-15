package kbuildbarn

import (
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/stretchr/testify/assert"
)

func TestByteStreamUrl(t *testing.T) {
	testCases := []struct {
		desc     string
		url      string
		wantHash string
		wantSize string
		wantErr  string
	}{
		{
			desc:     "tcp url",
			url:      "bytestream://build.local.enfabrica.net:8000/blobs/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
			wantHash: "a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a",
			wantSize: "465",
		},
		{
			desc:    "missing blobs path",
			url:     "bytestream://build.local.enfabrica.net:8000/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
			wantErr: "not well formed",
		},
		{
			desc:     "parses hash and size in opaque manner",
			url:      "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar",
			wantHash: "foo",
			wantSize: "bar",
		},
		{
			desc:    "missing hash and size elements",
			url:     "bytestream://build.local.enfabrica.net:8000",
			wantErr: "not well formed",
		},
		{
			desc:    "missing hash and size values",
			url:     "bytestream://build.local.enfabrica.net:8000////",
			wantErr: "not well formed",
		},
		{
			desc:     "unix socket address with .sock suffix",
			url:      "bytestream://////builder/home/.cache/buildbarn.sock/blobs/c633e871e139a8dc048cad45fcfd3f016c292cc479e0b37a472b285974f87182/1203284",
			wantHash: "c633e871e139a8dc048cad45fcfd3f016c292cc479e0b37a472b285974f87182",
			wantSize: "1203284",
		},
		{
			desc:    "unix socket address without .sock suffix",
			url:     "bytestream://////builder/home/.cache/buildbarn.foobar/blobs/c633e871e139a8dc048cad45fcfd3f016c292cc479e0b37a472b285974f87182/1203284",
			wantErr: "not well formed",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotHash, gotSize, gotErr := ParseByteStreamUrl(tc.url)

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}

			assert.Equal(t, tc.wantHash, gotHash)
			assert.Equal(t, tc.wantSize, gotSize)
		})
	}
}

func TestDefaultUrlGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := ParseByteStreamUrl(exampleUrl)
	assert.NoError(t, err)
	baseName := "buildbarn.local"
	assert.Equal(t, "http://buildbarn.local/blobs/action/foo-bar", Url(baseName, hash, size, WithActionUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/command/foo-bar", Url(baseName, hash, size, WithCommandUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/directory/foo-bar", Url(baseName, hash, size, WithDirectoryUrlTemplate()))
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/", Url(baseName, hash, size, WithFileName("")))
}

func TestFileUrlGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := ParseByteStreamUrl(exampleUrl)
	basename := "buildbarn.local"
	assert.NoError(t, err)
	assert.Equal(t, "http://buildbarn.local/blobs/file/foo-bar/mickey.mouse", Url(basename, hash, size, WithFileName("mickey.mouse")))
}
func TestByteStreamGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := ParseByteStreamUrl(exampleUrl)
	basename := "buildbarn.local"
	assert.NoError(t, err)
	assert.Equal(t, "bytestream://buildbarn.local/blobs/foo/bar", Url(basename, hash, size, WithByteStreamTemplate()))
}

func TestFileGeneration(t *testing.T) {
	exampleUrl := "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar"
	hash, size, err := ParseByteStreamUrl(exampleUrl)
	basename := "/root"
	assert.NoError(t, err)
	assert.Equal(t, "/root/blobs/file/foo-bar/foo.go", File(basename, hash, size, WithFileName("foo.go")))
}
