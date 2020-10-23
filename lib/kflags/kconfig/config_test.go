package kconfig

import (
	"flag"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/downloader"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
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
    }],
    "Command": [{
      "Name": "docker",
      "Use": "<image> [params]...",
      "Short": "Starts a docker container",
      "Flag": [{
        "Name": "version",
        "Help": "manage docker images",
        "Default": "stable"
      }],
      "Implementation": {
        "Var": [{
          "Key": "repository",
          "Value": "gcr.io"
        }],
        "Package": {
          "URL": "/package.tar.gz",
          "Hash": "b68f2a7936f57307fc46388cc5bebbf7052a5d97511b53f859a6badef84b1110"
        }
      }
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

	found, err := r.VisitFlag("", &kflags.GoFlag{Flag: sflag})
	assert.True(t, found)
	assert.Equal(t, "19", *fooserver)

	dl.Wait()
	r.Done()
}

func TestCommandAugmenter(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tempdir}
	dl, err := downloader.New()
	assert.Nil(t, err)

	mux, url, err := ktest.StartServer(ktest.StringHandler(jconfigRoot))
	assert.Nil(t, err)
	mux.HandleFunc("/astore.json", ktest.StringHandler(jconfigAstore))
	mux.HandleFunc("/package.tar.gz", ktest.CachableTestDataHandler("package.tar.gz"))

	r, err := NewConfigAugmenterFromURL(c, url, WithDownloader(dl), WithLogger(&logger.DefaultLogger{Printer: log.Printf}))
	assert.Nil(t, err)

	mc := &MockCommand{MyName: "root"}
	found, err := r.VisitCommand("", mc)
	assert.Nil(t, err, "%v", err)
	assert.True(t, found)
	assert.Equal(t, 1, len(mc.Sub))

	mc = &MockCommand{MyName: "astore"}
	found, err = r.VisitCommand("docker", mc)
	assert.Nil(t, err)
	assert.True(t, found)

	assert.Equal(t, 2, len(mc.Sub))
	start := mc.Sub[0]
	assert.Equal(t, "start", start.Definition.Name)
	assert.Equal(t, "use", start.Definition.Use)
	assert.Equal(t, "short", start.Definition.Short)
	assert.Equal(t, "long", start.Definition.Long)
	assert.Equal(t, "example", start.Definition.Example)
	assert.Equal(t, []string{"run", "go"}, start.Definition.Aliases)
	assert.Equal(t, 2, len(start.Flag))
	assert.Equal(t, "option1", start.Flag[0].Name)
	assert.Equal(t, "test optional parameter", start.Flag[0].Help)
	assert.Equal(t, "value1", start.Flag[0].Default)
	assert.Equal(t, "option2", start.Flag[1].Name)
	assert.Equal(t, "test optional parameter", start.Flag[1].Help)
	assert.Equal(t, "value2", start.Flag[1].Default)

	stop := mc.Sub[1]
	assert.Equal(t, "stop", stop.Definition.Name)
	assert.Equal(t, "don't use this", stop.Definition.Use)
	assert.Equal(t, "short description", stop.Definition.Short)
	assert.Equal(t, "long description", stop.Definition.Long)
	assert.Equal(t, "example", stop.Definition.Example)
	assert.Equal(t, 0, len(stop.Definition.Aliases))
	assert.Equal(t, 0, len(stop.Flag))

	// Note that the {{.truth}} argoment should NOT be expanded.
	// We don't want to expand arguments users are supplying on the CLI.
	err = start.Action(nil, []string{"{{.truth}} or die", "toast"})
	assert.Nil(t, err)
	// TODO: actually verify the action run correctly.
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

	found, err := r.VisitFlag("", &kflags.GoFlag{Flag: sflag})
	assert.True(t, found)
	assert.Equal(t, "19", *fooserver)

	found, err = r.VisitFlag("", &kflags.GoFlag{Flag: bflag})
	assert.True(t, found)
	assert.Equal(t, "42", *barserver)

	found, err = r.VisitFlag("", &kflags.GoFlag{Flag: uflag})
	assert.False(t, found)
	assert.Equal(t, "initialw", *unknown)

	dl.Wait()

	err = r.Done()
	assert.Nil(t, err, "%s", err)
}
