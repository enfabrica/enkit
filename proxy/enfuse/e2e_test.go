package enfuse_test

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/enfuse"
	enfuse_rpc "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	d, files := CreateSeededTmpDir(t, 10)
	p, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	a, err := p.Address()
	assert.Nil(t, err)
	fmt.Println(p.Addr().String())
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
	c := enfuse_rpc.NewFuseControllerClient(conn)
	r, err := c.FileInfo(context.TODO(), &enfuse_rpc.FileInfoRequest{Dir: ""})
	assert.NoError(t, err)
	assert.Equal(t, len(files)+1, len(r.Files))
	offset := uint64(1)
	for _, tmpf := range files {
		rr, err := c.Files(context.TODO(), &enfuse_rpc.RequestFile{
			Path:   tmpf.Name,
			Offset: offset,
		})
		assert.NoError(t, err)
		assert.Equal(t, tmpf.Data[offset:], rr.Content)
	}
}

func TestNewFuseShareCommand(t *testing.T) {
	d, _ := CreateSeededTmpDir(t, 10)
	p, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	a, err := p.Address()
	assert.Nil(t, err)
	fmt.Println(p.Addr().String())
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
	c := enfuse.FuseClient{ConnClient: enfuse_rpc.NewFuseControllerClient(conn)}
	err = os.Mkdir("hello", 0777)
	assert.NoError(t, err)
	assert.NoError(t, enfuse.MountDirectory("hello", &c))
	//m, err := fstestutil.MountedT(t, &c, nil, fuse.AllowOther())
	//assert.NoError(t, err)
	//fmt.Println("mounted at ", m.Dir)
	//defer m.Close()
	//dd, err := ioutil.ReadDir(m.Dir)
	//assert.NoError(t, err)
	//fmt.Println("yay", dd)
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
	content := []byte(strconv.Itoa(rng.Int()))
	_, err = f.Write(content)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	return TmpFile{
		Name: strings.ReplaceAll(filename, tmpDirName, ""),
		Data: content,
	}
}
