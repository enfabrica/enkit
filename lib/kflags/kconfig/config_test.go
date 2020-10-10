package kconfig

import (
	"flag"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/downloader"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

var jconfigAstore = `
{
  "Namespace": [{
    "Name": "",
    "Default": [{
      "Name": "foo-server",
      "Value": "14"
    }, {
      "Name": "bar-server",
      "Value": "42"
    }]
  }]
}`
var jconfigRoot = `
{
  "Include": [
    "/astore.json"
  ],
  "Namespace": [{
    "Name": "",
    "Default": [{
      "Name": "foo-server",
      "Value": "19"
    }]
  }]
}`

// By means of configuring a single handler in the HTTP server,
// the same page with the same include will be served over and over,
// creating a loop.
func TestConfigAugmenterLoop(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tempdir}
	dl, err := downloader.New()
	assert.Nil(t, err)

	_, url, err := ktest.StartServer(ktest.StringHandler(jconfigRoot))
	assert.Nil(t, err)

	r, err := NewConfigAugmenterFromURL(c, url, WithDownloader(dl))
	assert.Nil(t, err)

	fs := flag.NewFlagSet("", flag.PanicOnError)
	fooserver := fs.String("foo-server", "initials", "usage")
	sflag := fs.Lookup("foo-server")

	found, err := r.Visit("", &kflags.GoFlag{sflag})
	assert.True(t, found)
	assert.Equal(t, "19", *fooserver)

	dl.Wait()

	err = r.Done()
	// Turned a bunch of errors into warnings.
	// assert.NotNil(t, err, "%s", err)
}

func TestConfigAugmenter(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tempdir}
	dl, err := downloader.New()
	assert.Nil(t, err)

	mux, url, err := ktest.StartServer(ktest.StringHandler(jconfigRoot))
	assert.Nil(t, err)
	mux.HandleFunc("/astore.json", ktest.StringHandler(jconfigAstore))

	r, err := NewConfigAugmenterFromURL(c, url, WithDownloader(dl))
	assert.Nil(t, err)

	fs := flag.NewFlagSet("", flag.PanicOnError)
	fooserver := fs.String("foo-server", "initials", "usage")
	sflag := fs.Lookup("foo-server")

	barserver := fs.String("bar-server", "initialb", "usage")
	bflag := fs.Lookup("bar-server")

	unknown := fs.String("whatever-server", "initialw", "usage")
	uflag := fs.Lookup("whatever-server")

	found, err := r.Visit("", &kflags.GoFlag{sflag})
	assert.True(t, found)
	assert.Equal(t, "19", *fooserver)

	found, err = r.Visit("", &kflags.GoFlag{bflag})
	assert.True(t, found)
	assert.Equal(t, "42", *barserver)

	found, err = r.Visit("", &kflags.GoFlag{uflag})
	assert.False(t, found)
	assert.Equal(t, "initialw", *unknown)

	dl.Wait()

	err = r.Done()
	assert.Nil(t, err, "%s", err)
}
