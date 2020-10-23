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
	"net/url"
	"strings"
	"testing"
)

func getNamespaces(url string, invalid bool) []Namespace {
	namespaces := []Namespace{
		{
			Name: "",
			Default: []Parameter{
				{
					Name:  "astore-server",
					Value: "127.0.0.1",
				},
				{
					Name:  "astore-whatever",
					Value: "fuffa",
				},
				{
					Name:     "astore-certificate",
					Source:   SourceURL,
					Value:    "%URL%",
					Encoding: EncodeFile,
				},
			},
		},
		{
			Name: "whatever",
			Default: []Parameter{
				{
					Name:  "astore-whatever",
					Value: "42",
				},
			},
		},
		{
			Name: "astore",
			Default: []Parameter{
				{
					Name:  "astore-server",
					Value: "127.0.0.2",
				},
				{
					Name:     "astore-certificate",
					Value:    "inline-certificate",
					Encoding: EncodeBase64,
				},
				{
					// This is legal, could be an array value, with multiple sources.
					Name:  "astore-server",
					Value: "foo-bar",
				},
			},
			Command: []Command{
				{
					CommandDefinition: kflags.CommandDefinition{
						Name: "docker",
					},

					Flag: []kflags.FlagDefinition{
						{
							Name:    "test",
							Help:    "This is a test flag",
							Default: "freedom",
						},
					},

					Implementation: &Implementation{
						Package: &Package{
							URL:  "fake",
							Hash: "fake",
						},
					},
				},
			},
		},
	}

	bad := []Namespace{
		{
			// This is invalid, it should be ignored.
			Name: "astore",
			Default: []Parameter{
				{
					Name:  "astore-server",
					Value: "127.0.0.3",
				},
			},
		},
		{
			Name: "foobar",
			Default: []Parameter{
				{
					// Invalid definition.
					Value: "platipus",
				},
			},
		},
	}

	for nx, namespace := range namespaces {
		for px, parm := range namespace.Default {
			namespaces[nx].Default[px].Value = strings.ReplaceAll(parm.Value, "%URL%", url)
		}
	}

	if invalid {
		namespaces = append(namespaces, bad...)
	}
	return namespaces
}

func TestAugmenterWithError(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tempdir}
	dl, err := downloader.New()
	assert.Nil(t, err)

	http := ktest.Capture(ktest.CachableStringHandler(message))
	_, url, err := ktest.StartServer(http.Handle)
	assert.Nil(t, err)

	namespaces := getNamespaces(url, true)
	_, err = NewNamespaceAugmenter(nil, namespaces, nil, nil, nil, NewCreator(logger.Nil, c, dl).Create)
	assert.NotNil(t, err, "%s", err)
}

type MockSubCommand struct {
	Definition kflags.CommandDefinition
	Flag       []kflags.FlagDefinition
	Action     kflags.CommandAction
}

type MockCommand struct {
	MyName string
	Hidden bool
	Sub    []MockSubCommand
}

func (mc *MockCommand) Name() string {
	return mc.MyName
}

func (mc *MockCommand) Hide(value bool) {
	mc.Hidden = value
}

func (mc *MockCommand) AddCommand(def kflags.CommandDefinition, fl []kflags.FlagDefinition, action kflags.CommandAction) error {
	mc.Sub = append(mc.Sub, MockSubCommand{
		Definition: def,
		Flag:       fl,
		Action:     action,
	})
	return nil
}

func TestAugmenterCommand(t *testing.T) {
	retrieved := []string{}
	mockRetrieve := func(url, hash string) (string, *Manifest, error) {
		retrieved = append(retrieved, url)
		return "", nil, nil
	}

	created := []*Parameter{}
	mockCreator := func(url *url.URL, param *Parameter) (Retriever, error) {
		created = append(created, param)
		return nil, nil
	}

	namespaces := getNamespaces("http://non-existant-url/", false)
	ag, err := NewNamespaceAugmenter(nil, namespaces, nil, nil, mockRetrieve, mockCreator)

	// First round: the test namespace adds a subcommand to astore, but
	// does not cause any retrieval, as the entire configuration is self contained.
	mc := &MockCommand{MyName: "test"}
	done, err := ag.VisitCommand("astore", mc)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(retrieved), "%v", retrieved)
	assert.Equal(t, 1, len(mc.Sub))

	// Second round: we now have a command that needs retreival.
	// Let's visit it, and see what happens.
	mc = &MockCommand{MyName: "test"}
	done, err = ag.VisitCommand("astore.docker", mc)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(retrieved), "%v", retrieved)
	assert.Equal(t, 0, len(mc.Sub))
}

func TestAugmenter(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "cache")
	assert.Nil(t, err)
	c := &cache.Local{Root: tempdir}
	dl, err := downloader.New()
	assert.Nil(t, err)

	http := ktest.Capture(ktest.CachableStringHandler(message))
	_, url, err := ktest.StartServer(http.Handle)
	assert.Nil(t, err)

	namespaces := getNamespaces(url, false)
	r, err := NewNamespaceAugmenter(nil, namespaces, nil, nil, nil, NewCreator(logger.Nil, c, dl).Create)
	assert.Nil(t, err, "%s", err)

	server := flag.String("astore-server", "initials", "usage")
	sflag := flag.Lookup("astore-server")

	certificate := flag.String("astore-certificate", "initialc", "usage")
	cflag := flag.Lookup("astore-certificate")

	// invalid namespace, the flag is not found.
	found, err := r.VisitFlag("invalid", &kflags.GoFlag{Flag: sflag})
	derr := r.Done()

	assert.Nil(t, derr)
	assert.Nil(t, err)
	assert.False(t, found)
	assert.Equal(t, *server, "initials")

	// Flag should be found and applied.
	found, err = r.VisitFlag("", &kflags.GoFlag{Flag: sflag})
	derr = r.Done()

	assert.Nil(t, derr)
	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, "127.0.0.1", *server)

	// Same as above, the value should be set to a cached path.
	found, err = r.VisitFlag("", &kflags.GoFlag{Flag: cflag})
	derr = r.Done()

	assert.Nil(t, derr, "%s", derr)
	assert.Nil(t, err, "%s", err)
	assert.True(t, found)
	data, err := ioutil.ReadFile(*certificate)
	assert.Nil(t, err)
	assert.Equal(t, message, string(data))

	// Value should now be inlined.
	found, err = r.VisitFlag("astore", &kflags.GoFlag{Flag: cflag})
	derr = r.Done()
	assert.Nil(t, derr, "%s", derr)
	assert.Nil(t, err, "%s", err)
	assert.True(t, found)
	assert.Equal(t, "aW5saW5lLWNlcnRpZmljYXRl", *certificate)

	unknown := flag.Int("astore-whatever", 14, "usage")
	uflag := flag.Lookup("astore-whatever")

	found, err = r.VisitFlag("astore", &kflags.GoFlag{Flag: uflag})
	derr = r.Done()
	assert.Nil(t, derr, "%s", derr)
	assert.Nil(t, err, "%s", err)
	assert.False(t, found)
	assert.Equal(t, 14, *unknown)

	found, err = r.VisitFlag("whatever", &kflags.GoFlag{Flag: uflag})
	derr = r.Done()
	assert.Nil(t, derr, "%s", derr)
	assert.Nil(t, err, "%s", err)
	assert.True(t, found)
	assert.Equal(t, 42, *unknown)

	found, err = r.VisitFlag("", &kflags.GoFlag{Flag: uflag})
	derr = r.Done()
	assert.NotNil(t, derr, "%s", derr)
	assert.Nil(t, err, "%s", err)
	assert.True(t, found)
}
