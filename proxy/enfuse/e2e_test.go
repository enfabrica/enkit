package enfuse_test

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs/fstestutil"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFuseShareEncryption(t *testing.T) {
	d, generatedFiles := CreateSeededTmpDir(t, 5)
	p, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	a, err := p.Address()
	assert.Nil(t, err)
	pubChan := make(chan *enfuse.ClientEncryptionInfo, 1)
	// these are the server dns names that get embedded in the root CA. while arbitrary, it's best practice to at least us
	// a real dns name. Since this is p2p or port-forwarded, in prod this will also be localhost
	serverDnsNames := []string{"localhost"}
	// same reason as above
	serverIpAddresses := []net.IP{net.ParseIP("127.0.0.1")}
	scfg := &enfuse.ConnectConfig{
		Url:         "127.0.0.1",
		Port:        a.Port,
		DnsNames:    serverDnsNames,
		IpAddresses: serverIpAddresses,
	}
	s := enfuse.NewServer(
		enfuse.NewServerConfig(
			enfuse.WithDir(d),
			enfuse.WithEncryption(pubChan),
			enfuse.WithConnectMods(
				enfuse.WithConnectConfig(scfg),
			),
		),
	)
	go func() {
		assert.Nil(t, s.Serve())
	}()
	clientEncryptionInfo := <-pubChan
	time.Sleep(10 * time.Millisecond)
	cfg := &enfuse.ConnectConfig{
		Url:         "127.0.0.1",
		Port:        a.Port,
		DnsNames:    serverDnsNames,
		IpAddresses: serverIpAddresses,
	}
	err = cfg.ApplyClientEncryptionInfo(clientEncryptionInfo)
	assert.NoError(t, err)
	c, err := enfuse.NewClient(cfg)
	assert.NoError(t, err)
	t.Run("Test With Encryption", func(t *testing.T) {
		testFile(t, c, generatedFiles)
	})
}

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
	c, err := enfuse.NewClient(&enfuse.ConnectConfig{Port: a.Port, Url: "127.0.0.1"})
	m, err := fstestutil.MountedT(t, c, nil)
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
	assert.NoError(t, err)
	t.Run("Sanity Test", func(t *testing.T) {
		testFile(t, c, generatedFiles)
	})
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

func testFile(t *testing.T, c *enfuse.FuseClient, generatedFiles []TmpFile) {
	m, err := fstestutil.MountedT(t, c, nil, fuse.AllowOther())
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
