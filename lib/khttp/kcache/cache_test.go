package kcache

import (
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

// Do the same Get twice. On the second run, the file should be in cache.
// The If-Modified-Since header should be set.
func TestFileIsCached(t *testing.T) {
	recorder := ktest.Capture(ktest.HelloHandler)
	_, url, err := ktest.StartServer(recorder.Handle)
	assert.Nil(t, err)

	td, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)

	local := &cache.Local{Root: td}

	data := ""
	err = protocol.Get(url, protocol.Read(protocol.String(&data)), WithCache(local))
	assert.Nil(t, err)
	assert.Equal(t, "hello", data)

	err = protocol.Get(url, protocol.Read(protocol.String(&data)), WithCache(local))
	assert.Nil(t, err)
	assert.Equal(t, "hello", data)

	assert.Equal(t, 2, len(recorder.Request))
	assert.Equal(t, "", recorder.Request[0].Header.Get("If-Modified-Since"))
	assert.Equal(t, "", recorder.Response[0].Header.Get("Last-Modified"))
	assert.Equal(t, http.StatusOK, recorder.Response[0].StatusCode)
	assert.Regexp(t, "...,.* GMT", recorder.Request[1].Header.Get("If-Modified-Since"))
	assert.Equal(t, http.StatusOK, recorder.Response[1].StatusCode)
}

// Multiple gets for the same file. HTTP handler serves the content correctly, 2nd, 3rd request
// should result in 304 status. Verify the status.
func TestLastModified(t *testing.T) {
	recorder := ktest.Capture(ktest.CachableHelloHandler)
	_, url, err := ktest.StartServer(recorder.Handle)
	assert.Nil(t, err)

	local := &cache.Local{Root: "."}

	for i := 0; i < 3; i++ {
		data := ""
		err = protocol.Get(url, protocol.Read(protocol.String(&data)), WithCache(local))
		assert.Nil(t, err, "error %s", err)
		assert.Equal(t, "hello", data)
	}

	assert.Equal(t, 3, len(recorder.Request))

	assert.Equal(t, "", recorder.Request[0].Header.Get("If-Modified-Since"))
	assert.Equal(t, "Thu, 01 Jan 1970 00:00:10 GMT", recorder.Response[0].Header.Get("Last-Modified"))
	assert.Equal(t, http.StatusOK, recorder.Response[0].StatusCode)

	assert.Regexp(t, "Thu, 01 Jan 1970 00:00:10 GMT", recorder.Request[1].Header.Get("If-Modified-Since"))
	assert.Equal(t, "Thu, 01 Jan 1970 00:00:10 GMT", recorder.Response[1].Header.Get("Last-Modified"))
	assert.Equal(t, http.StatusNotModified, recorder.Response[1].StatusCode)

	assert.Regexp(t, "Thu, 01 Jan 1970 00:00:10 GMT", recorder.Request[2].Header.Get("If-Modified-Since"))
	assert.Equal(t, "Thu, 01 Jan 1970 00:00:10 GMT", recorder.Response[2].Header.Get("Last-Modified"))
	assert.Equal(t, http.StatusNotModified, recorder.Response[2].StatusCode)

	// Check that the cache papers over a 500 error.
	recorder.Handler = ktest.ErrorHandler
	data := ""
	err = protocol.Get(url, protocol.Read(protocol.String(&data)), WithCache(local))
	assert.Nil(t, err, "error %s", err)
	assert.Equal(t, "hello", data)

	// Check that the cache papers over a timeout.
	recorder.Handler = ktest.HangingHandler
	data = ""
	err = protocol.Get(url, protocol.Read(protocol.String(&data)), protocol.WithTimeout(100*time.Millisecond), WithCache(local))
	assert.Nil(t, err, "error %s", err)
	assert.Equal(t, "hello", data)
}
