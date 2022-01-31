package amux_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/enfabrica/enkit/proxy/amux"
	"github.com/enfabrica/enkit/proxy/amux/amuxie"
	"github.com/stretchr/testify/assert"
)

func CountHandler(counter *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*counter += 1
	})
}

var NilHandler = http.HandlerFunc(
	func(w http.ResponseWriter, r *http.Request) {
	},
)

func Request(m amux.Mux, host, path string) int {
	r := httptest.NewRequest("GET", fmt.Sprintf("http://%s%s", host, path), nil)
	w := httptest.NewRecorder()

	h := m.(http.Handler)
	h.ServeHTTP(w, r)

	resp := w.Result()
	return resp.StatusCode
}

func TestMuxConformance(t *testing.T) {
	var m amux.Mux

	// TODO: if support for multiple muxes is added, the test here should pass
	// on all added muxes. An error means the new mux introduces a backward
	// incompatible change into how routes are treated.
	m = amuxie.New()
	assert.NotNil(t, m)

	m.Handle("/quote", NilHandler)
	m.Handle("/exactdir/", NilHandler)

	var countPrefix, countOverlap int
	m.Handle("/prefix/*", CountHandler(&countPrefix))
	m.Handle("/tooverlap/", CountHandler(&countOverlap))

	h1 := m.Host("host1.net")

	h1.Handle("/", NilHandler)
	h1.Handle("/separate", NilHandler)

	var countBar, countFoo, countOverride int
	h1.Handle("/prefix/bar/", CountHandler(&countBar))
	h1.Handle("/prefix/foo/*", CountHandler(&countFoo))
	h1.Handle("/tooverlap/", CountHandler(&countOverride))

	h2 := m.Host("host2.net")
	var countHost2 int
	h2.Handle("/*", CountHandler(&countHost2))

	assert.Equal(t, http.StatusNotFound, Request(m, "whatever", "/"))

	assert.Equal(t, http.StatusOK, Request(m, "whatever", "/quote"))
	assert.Equal(t, http.StatusOK, Request(m, "whatever.", "/quote"))
	assert.Equal(t, http.StatusNotFound, Request(m, "whatever", "/quote/sub"))

	assert.Equal(t, http.StatusOK, Request(m, "whatever", "/exactdir"))
	assert.Equal(t, http.StatusNotFound, Request(m, "whatever", "/exactdir/foo"))
	assert.Equal(t, http.StatusNotFound, Request(m, "whatever", "/exactdir/"))

	assert.Equal(t, http.StatusOK, Request(m, "whatever", "/prefix/"))
	assert.Equal(t, http.StatusOK, Request(m, "whatever.", "/prefix/test1"))
	assert.Equal(t, http.StatusOK, Request(m, "whatever.", "/prefix/test1/test2/test3"))
	assert.Equal(t, http.StatusOK, Request(m, "whatever.", "/prefix/bar/test2/test3"))
	assert.Equal(t, http.StatusNotFound, Request(m, "whatever", "/prefix"))

	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net", "/not-found"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/separate"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net.", "/separate"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net.", "/separate/"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net.", "/separate/foo"))

	assert.Equal(t, http.StatusOK, Request(m, "non-existing", "/tooverlap"))
	assert.Equal(t, http.StatusNotFound, Request(m, "non-existing", "/tooverlap/"))
	assert.Equal(t, 1, countOverlap)
	assert.Equal(t, 0, countOverride)
	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/tooverlap"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net.", "/tooverlap"))
	assert.Equal(t, 1, countOverlap)
	assert.Equal(t, 2, countOverride)

	countPrefix = 0
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net", "/prefix/bar/whatever"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net.", "/prefix/bar/whatever"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net", "/prefix/foo"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net.", "/prefix/foo"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net", "/prefix/nanna"))
	assert.Equal(t, http.StatusNotFound, Request(m, "host1.net.", "/prefix/nanna"))
	assert.Equal(t, 0, countPrefix)
	assert.Equal(t, 0, countBar)
	assert.Equal(t, 0, countFoo)

	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/prefix/bar"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net.", "/prefix/bar"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/prefix/foo/test/toast"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net.", "/prefix/foo/test/toast"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net", "/prefix/foo/"))
	assert.Equal(t, http.StatusOK, Request(m, "host1.net.", "/prefix/foo/"))
	assert.Equal(t, 0, countPrefix)
	assert.Equal(t, 2, countBar)
	assert.Equal(t, 4, countFoo)

	assert.Equal(t, 0, countHost2)
	assert.Equal(t, http.StatusOK, Request(m, "host2.net", "/"))
	assert.Equal(t, http.StatusOK, Request(m, "host2.net.", "/whatever"))
	assert.Equal(t, 2, countHost2)
}
