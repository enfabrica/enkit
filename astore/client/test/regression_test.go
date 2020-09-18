package test

import (
	"flag"
	"fmt"
	"github.com/enfabrica/enkit/astore/client/astore"
	aserver "github.com/enfabrica/enkit/astore/server/astore"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/config/directory"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

var bf *client.BaseFlags
var af *client.ServerFlags
var rng *rand.Rand

// Go and bazel tests run in an hermetic environment.
// This is necessary so that we can load the user credentials without having the HOME environment variable.
func Open(name string, namespace ...string) (config.Store, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	dir, err := directory.OpenDir(user.HomeDir, append([]string{".config", name}, namespace...)...)
	if err != nil {
		return nil, err
	}

	return config.NewMulti(dir), err
}

func Store() (*astore.Client, error) {
	_, cookie, err := bf.IdentityCookie()
	if err != nil {
		return nil, err
	}
	conn, err := af.Connect(client.WithCookie(cookie))
	return astore.New(conn), err
}

func TestMain(m *testing.M) {
	rng = rand.New(srand.Source)

	set := &kflags.GoFlagSet{FlagSet: flag.CommandLine}

	af = client.DefaultServerFlags("store", "Artifacts store metadata server", "http://127.0.0.1:6433/api/grpc")
	af.Register(set, "enkit.")

	bf = client.DefaultBaseFlags("test", "enkit")
	bf.ConfigOpener = Open
	bf.Register(set, "enkit.")

	flag.Parse()
	bf.Init()

	os.Exit(m.Run())
}

func Unique(pattern string) string {
	uid, err := aserver.GenerateUid(rng)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(pattern, uid)
}

func TestUid(t *testing.T) {
	for i := 0; i < 1000; i++ {
		uid, err := aserver.GenerateUid(rng)
		assert.Nil(t, err, "%v", err)
		assert.True(t, astore.IsUid(uid))
	}
}

func TestSimple(t *testing.T) {
	ac, err := Store()
	if !assert.Nil(t, err, "error connecting to test astore %s", err) {
		return
	}
	if !assert.NotNil(t, ac) {
		return
	}

	// Upload 3 copies of the same file.
	name := Unique("testadata/%s/uploaded.txt")
	files := []astore.FileToUpload{
		{
			Local:  filepath.Join("testdata", "file.txt"),
			Remote: name,
			Note:   "simple upload",
			Tag:    []string{"simple"},
		},
		{
			Local:  filepath.Join("testdata", "file.txt"),
			Remote: name,
			Note:   "simple upload",
			Tag:    []string{"simple"},
		},
		// Last element does not have tag simple! This will cause
		// a) the simple tag to move once, and b) different elements
		// having the "simple" and "latest" tag (verified below).
		{
			Local:  filepath.Join("testdata", "file.txt"),
			Remote: name,
			Note:   "simple upload",
		},
	}

	ctx := bf.Context()
	o := astore.UploadOptions{
		Context: ctx,
	}
	res, err := ac.Upload(files, o)
	assert.Nil(t, err, "%v", err)
	assert.NotNil(t, 3, len(res))

	uids := map[string]interface{}{}
	sids := map[string]interface{}{}
	for _, r := range res {
		_, fuid := uids[r.Uid]
		assert.False(t, fuid, "uid already seen?? %s", r.Uid)
		_, fsid := sids[r.Sid]
		assert.False(t, fsid, "sid already seen?? %s", r.Sid)

		sids[r.Sid] = struct{}{}
		uids[r.Uid] = struct{}{}

		assert.NotEmpty(t, r.Sid)
		assert.NotEmpty(t, r.Uid)
		assert.Equal(t, 16, len(r.MD5))
		assert.Equal(t, "all", r.Architecture)
	}

	// No tag specified, returns all the elements.
	allarts, els, err := ac.List(name, astore.ListOptions{
		Context: ctx,
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(els), "%#v", els)
	assert.Equal(t, 3, len(allarts), "%#v", allarts)

	// Verify that only one of the elements has the "latest" tag, and only
	// one of the elements has the "simple" tag.
	found := []string{}
	for _, art := range allarts {
		if art.Tag == nil {
			continue
		}

		for _, t := range art.Tag {
			found = append(found, t)
		}
	}
	assert.ElementsMatch(t, []string{"simple", "latest"}, found)

	// One tag specified, returns one element.
	arts, els, err := ac.List(name, astore.ListOptions{
		Context: ctx,
		Tag:     []string{"simple"},
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(els), "%#v", els)
	assert.Equal(t, 1, len(arts), "%#v", arts)
	simple := arts[0]

	// One tag specified, returns one element.
	arts, els, err = ac.List(name, astore.ListOptions{
		Context: ctx,
		Tag:     []string{"latest"},
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(els), "%#v", els)
	assert.Equal(t, 1, len(arts), "%#v", arts)
	latest := arts[0]

	assert.NotEqual(t, simple.Sid, latest.Sid)
	assert.NotEqual(t, simple.Uid, latest.Uid)

	// Download one of the files.
	arts, err = ac.Download([]astore.FileToDownload{{
		Remote: name,
		Local:  "result.txt",
	}}, astore.DownloadOptions{
		Context: ctx,
	})
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, 1, len(arts))
	// No architecture, no version, should mean "latest" and "all".
	assert.Equal(t, []string{"latest"}, arts[0].Tag)
	assert.Equal(t, "all", arts[0].Architecture)

	// Download it again. Should fail, no overwrite allowed.
	arts, err = ac.Download([]astore.FileToDownload{
		{
			Remote: name,
			Local:  "result.txt",
		},
	}, astore.DownloadOptions{
		Context: ctx,
	})
	assert.NotNil(t, err)

	// Download it one more time. Set ovewrite. Pick a different tag.
	arts, err = ac.Download([]astore.FileToDownload{{
		Remote:    name,
		Local:     "result.txt",
		Overwrite: true,
		Tag:       &[]string{"simple"},
	}}, astore.DownloadOptions{
		Context: ctx,
	})
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, 1, len(arts))
	assert.Equal(t, []string{"simple"}, arts[0].Tag)
	assert.Equal(t, "all", arts[0].Architecture)

	// Try to download something that does not exist.
	arts, err = ac.Download([]astore.FileToDownload{{
		Remote:    name,
		Local:     "result.txt",
		Overwrite: true,
		Tag:       &[]string{"simple", "latest"},
	}}, astore.DownloadOptions{
		Context: ctx,
	})
	assert.NotNil(t, err)

	// Try to download the oldest file by UID.
	// Specify no name, just for fun.
	arts, err = ac.Download([]astore.FileToDownload{{
		Remote:    allarts[0].Uid,
		Overwrite: true,
		// Should be ignored, when querying by Uid.
		Tag: &[]string{"simple", "latest"},
	}}, astore.DownloadOptions{
		Context: ctx,
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(arts))
}
