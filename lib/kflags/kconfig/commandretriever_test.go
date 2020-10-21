package kconfig

import (
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestCommandRetrieverHash(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tmpdir}

	cr := NewCommandRetriever(&logger.DefaultLogger{Printer: t.Logf}, c, retry.Nil)
	assert.NotNil(t, cr)

	http := ktest.Capture(ktest.CachableTestDataHandler("empty.tar.gz"))
	_, url, err := ktest.StartServer(http.Handle)
	url += "empty.tar.gz"

	dir1, err := cr.PrepareHash(url, "test-hash")
	assert.Nil(t, err)
	dir2, err := cr.PrepareHash(url, "test-hash")
	assert.Equal(t, dir1, dir2, "%s != %s", dir1, dir2)

	// Given that the url was retrieved by hash, there is no need to fetch it multiple
	// times, given that we already have the hash on disk.
	assert.Equal(t, 1, len(http.Request))
}

func TestCommandRetrieverHashError(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tmpdir}

	cr := NewCommandRetriever(&logger.DefaultLogger{Printer: t.Logf}, c, retry.Nil)
	assert.NotNil(t, cr)

	_, url, err := ktest.StartServer(ktest.ErrorHandler)
	url += "empty.tar.gz"

	_, err = cr.PrepareHash(url, "test-hash")
	assert.NotNil(t, err, "%v", err)
}

func TestCommandRetrieverURL(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tmpdir}

	cr := NewCommandRetriever(&logger.DefaultLogger{Printer: t.Logf}, c, retry.Nil)
	assert.NotNil(t, cr)

	http := ktest.Capture(ktest.CachableTestDataHandler("empty.tar.gz"))
	_, url, err := ktest.StartServer(http.Handle)
	url += "empty.tar.gz"

	// Two requests, but the second one should return an If-Modified-Since that indicates
	// that the file has not changed, thus we should get the same result twice.
	dir1, err := cr.PrepareURL(url)
	assert.Nil(t, err)
	dir2, err := cr.PrepareURL(url)
	assert.Nil(t, err)

	assert.Equal(t, dir1, dir2, "%s != %s", dir1, dir2)
	assert.Equal(t, 2, len(http.Request))
	assert.Equal(t, "304 Not Modified", http.Response[1].Status)
}

func TestCommandRetrieverURLNotCached(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tmpdir}

	cr := NewCommandRetriever(&logger.DefaultLogger{Printer: t.Logf}, c, retry.Nil)
	assert.NotNil(t, cr)

	http := ktest.Capture(ktest.TestDataHandler("empty.tar.gz"))
	_, url, err := ktest.StartServer(http.Handle)
	url += "empty.tar.gz"

	// No caching, both requests will result in downloading a file.
	_, err = cr.PrepareURL(url)
	assert.Nil(t, err)
	_, err = cr.PrepareURL(url)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(http.Request))
	assert.Equal(t, "200 OK", http.Response[1].Status)
	assert.Equal(t, "200 OK", http.Response[0].Status)
}
