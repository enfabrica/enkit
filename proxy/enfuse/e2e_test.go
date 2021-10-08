package enfuse_test

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs/fstestutil"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/enfuse"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewFuseShareCommand(t *testing.T) {
	d, generatedFiles := CreateSeededTmpDir(t, 2)
	p, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	a, err := p.Address()
	assert.Nil(t, err)
	s := enfuse.NewServer(
		enfuse.NewServerConfig(
			enfuse.WithDir(d),
			enfuse.WithConnectMods(
				enfuse.WithListener(p),
			),
		),
	)
	go func() {
		assert.Nil(t, s.Serve())
	}()
	time.Sleep(5 * time.Millisecond)
	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", a.Port), grpc.WithInsecure())
	assert.Nil(t, err)
	defer conn.Close()
	c := enfuse.FuseClient{ConnClient: fusepb.NewFuseControllerClient(conn)}
	m, err := fstestutil.MountedT(t, &c, nil, fuse.AllowOther())
	assert.NoError(t, err)
	defer m.Close()

	var fusePaths []string
	assert.NoError(t, filepath.Walk(m.Dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fusePaths = append(fusePaths, path)
			assert.Greater(t, int(info.Size()), 0)
		}
		return err
	}))

	assert.Equal(t, len(generatedFiles), len(fusePaths))
	for _, genFile := range generatedFiles {
		for _, realFile := range fusePaths {
			if realFile == genFile.Name {
				btes, err := ioutil.ReadFile(realFile)
				assert.NoError(t, err)
				assert.Equal(t, len(genFile.Data), len(btes))
				assert.Truef(t, reflect.DeepEqual(btes, genFile.Data), "dta returned by fs equal")
			}
		}
	}
}

type TmpFile struct {
	Name string
	Data []byte
}

func CreateSeededTmpDir(t *testing.T, num int) (string, []TmpFile) {
	tmpDirName, err := os.MkdirTemp(os.TempDir(), "*")
	assert.Nil(t, err)
	var tts []TmpFile
	for i := 0; i < num; i++ {
		tts = append(tts, createTmpFile(t, tmpDirName))
	}
	return tmpDirName, tts
}

func createTmpFile(t *testing.T, tmpDirName string) TmpFile {
	rng := rand.New(srand.Source)
	cwd := tmpDirName
	for i := 0; i < rng.Intn(5); i++ {
		name, err := os.MkdirTemp(cwd, "*")
		assert.NoError(t, err)
		cwd = name
	}
	f, err := os.CreateTemp(cwd, "*.txt")
	assert.Nil(t, err)
	filename := f.Name()
	sizeOfFile := 1024 * 1024 * (rng.Intn(2) + 1) // size of the file is greater than rpc data.
	content := make([]byte, sizeOfFile)
	i, err := rng.Read(content)
	assert.NoError(t, err)
	assert.Equal(t, sizeOfFile, i)
	_, err = f.Write(content)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	return TmpFile{
		Name: strings.ReplaceAll(filename, tmpDirName, ""),
		Data: content,
	}
}
