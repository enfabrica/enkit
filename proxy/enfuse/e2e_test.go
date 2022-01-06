package enfuse_test

import (
	"bazil.org/fuse/fs/fstestutil"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/enfuse"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"github.com/enfabrica/enkit/proxy/enfuse/testserver"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManyClientsUsingBasicPeering(t *testing.T) {
	relayServer := testserver.NewWebSocketBasicClientServer(t)
	defer relayServer.Close()
	sharingPeerWssConn, _, err := websocket.DefaultDialer.Dial(strings.ReplaceAll(relayServer.URL+"/server", "http", "ws"), nil)
	assert.NoError(t, err)

	sharingPeerPort, err := knetwork.AllocatePort()
	assert.Nil(t, err)

	_ = enfuse.NewWebsocketTCPShim(enfuse.DefaultPayloadStrategy, sharingPeerPort.Listener, sharingPeerWssConn)
	d, generatedFiles := CreateSeededTmpDir(t, 2)
	assert.Nil(t, err)
	s := enfuse.NewServer(
		enfuse.NewServerConfig(
			enfuse.WithDir(d),
			enfuse.WithConnectMods(
				enfuse.WithListener(sharingPeerPort.Listener),
			),
		),
	)

	go func() {
		assert.Nil(t, s.Serve())
	}()
	wg := &sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		consumingPeerShim := generateConsumingPeerShim(t, relayServer)
		c, err := enfuse.NewClient(&enfuse.ConnectConfig{GrpcDialOpts: []grpc.DialOption{consumingPeerShim.DialOpt(), grpc.WithInsecure()}})
		assert.NoError(t, err)
		go ReadWriteClientFilesSubTest(t, c, generatedFiles, wg)
	}
	wg.Wait()
	relayServer.Close()
}

func generateConsumingPeerShim(t *testing.T, relayServer *httptest.Server) *enfuse.SocketShim {
	consumingPeerWssConn, _, err := websocket.DefaultDialer.Dial(strings.ReplaceAll(relayServer.URL+"/client", "http", "ws"), nil)
	assert.NoError(t, err)

	consumingPeerShim, err := enfuse.NewSocketShim(enfuse.DefaultPayloadStrategy, enfuse.NewWebsocketLock(consumingPeerWssConn))
	assert.NoError(t, err)
	return consumingPeerShim
}

func TestSingleClientUsingBasicPeering(t *testing.T) {
	relayServer := testserver.NewWebSocketBasicClientServer(t)
	defer relayServer.Close()
	sharingPeerWssConn, _, err := websocket.DefaultDialer.Dial(strings.ReplaceAll(relayServer.URL+"/server", "http", "ws"), nil)
	assert.NoError(t, err)

	sharingPeerPort, err := knetwork.AllocatePort()
	assert.Nil(t, err)

	_ = enfuse.NewWebsocketTCPShim(enfuse.DefaultPayloadStrategy, sharingPeerPort.Listener, sharingPeerWssConn)
	d, generatedFiles := CreateSeededTmpDir(t, 2)
	assert.Nil(t, err)
	s := enfuse.NewServer(
		enfuse.NewServerConfig(
			enfuse.WithDir(d),
			enfuse.WithConnectMods(
				enfuse.WithListener(sharingPeerPort.Listener),
			),
		),
	)

	go func() {
		assert.Nil(t, s.Serve())
	}()

	consumingPeerShim := generateConsumingPeerShim(t, relayServer)
	c, err := enfuse.NewClient(&enfuse.ConnectConfig{GrpcDialOpts: []grpc.DialOption{consumingPeerShim.DialOpt(), grpc.WithInsecure()}})
	assert.NoError(t, err)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ReadWriteClientFilesSubTest(t, c, generatedFiles, wg)
	wg.Wait()
	relayServer.Close()
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
	c := enfuse.FuseClient{ConnClient: fusepb.NewFuseControllerClient(conn)}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ReadWriteClientFilesSubTest(t, &c, generatedFiles, wg)
	wg.Wait()
}

func ReadWriteClientFilesSubTest(t *testing.T, consumingPeer *enfuse.FuseClient, generatedFiles []TmpFile, wg *sync.WaitGroup) {
	defer wg.Done()
	m, err := fstestutil.MountedT(t, consumingPeer, nil)
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
				assert.Truef(t, reflect.DeepEqual(btes, genFile.Data), "data returned by fs equal")
			}
		}
	}
}

type TmpFile struct {
	Name string
	Data []byte
}
// TODO(adam): speed this up
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
