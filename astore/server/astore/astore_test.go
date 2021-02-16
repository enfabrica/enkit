package astore

import (
	"context"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/client/astore"
	astore2 "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/progress"
	"io/ioutil"
	//"github.com/enfabrica/enkit/astore/rpc/auth"
	//auth2 "github.com/enfabrica/enkit/astore/server/auth"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"log"
	"math/rand"
	"net"
	"os"
	"testing"
)

func TestSid(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for i := 0; i < 1000; i++ {
		value, err := GenerateSid(rng)
		assert.Nil(t, err)
		assert.Equal(t, 34, len(value), "value: %s", value)
	}
}

func TestUid(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for i := 0; i < 1000; i++ {
		value, err := GenerateUid(rng)
		assert.Nil(t, err)
		assert.Equal(t, 32, len(value), "value: %s", value)
		assert.True(t, astore.IsUid(value))
	}
}

//TODO fix client so that it's signed urls can depend on an interface for actual e2e testing
func TestServer(t *testing.T) {
	os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")
	err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9000")
	if err != nil {
		t.Fatal(err.Error())
	}
	os.Getenv("STORAGE_EMULATOR_HOST")
	b, err := ioutil.ReadFile("./testdata/dummy.json")
	if err != nil {
		t.Fatal(err.Error())
	}

	server, err := New(rand.New(srand.Source),
		WithCredentialsJSON(b),
		WithSigningJSON(b),
		WithBucket("example-bucket"))

	if err != nil {
		t.Fatal(err.Error())
	}
	buffListener := bufconn.Listen(2048 * 2048)
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return buffListener.Dial()
	}
	grpcServer := grpc.NewServer()
	astore2.RegisterAstoreServer(grpcServer, server)
	//authServer, err := auth2.New(
	//	rand.New(srand.Source),
	//	auth2.WithAuthURL("http://empty"))
	//auth.RegisterAuthServer(grpcServer, authServer)
	go func() {
		if err := grpcServer.Serve(buffListener); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	conn, err := grpc.DialContext(context.Background(),
		"empty", grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure())

	if err != nil {
		t.Fatal(err.Error())
	}
	//running this as test ping feature
	client := astore.New(conn)
	res, _, err := client.List("/test", astore.ListOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Printf("list response is +%v \n", res)
	b, err = ioutil.ReadFile("./testdata/example.yaml")
	if err != nil {
		t.Fatal(err.Error())
	}
	fmt.Println("bytes are ", string(b))
	uploadFiles := []astore.FileToUpload{
		{Local: "./testdata/example.yaml"},
	}
	ctxWithLogger := ccontext.DefaultContext()
	ctxWithLogger.Logger = logger.DefaultLogger{Printer: log.Printf}
	ctxWithLogger.Progress = progress.NewDiscard

	uploadOption := astore.UploadOptions{
		Context: ctxWithLogger,
	}
	u, err := client.Upload(uploadFiles, uploadOption)
	if err != nil {
		t.Error(err.Error())
		fmt.Println("erroring in client upload")
	}

	fmt.Printf("upload is +%v \n", u)
	storeResponse, err := server.Store(context.Background(), &astore2.StoreRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if storeResponse.GetSid() == "" || storeResponse.GetUrl() == "" {
		t.Fatal(errors.New("invalid store response"))
	}
	resp, err := server.Commit(context.Background(), &astore2.CommitRequest{
		Sid:          storeResponse.GetSid(),
		Architecture: "dwarvenx99",
		Path:         "127.0.0.1:9000/hello/work/example.yaml",
		Note:         "note",
		Tag:          []string{"something"},
	})
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Println("finalizzing +%v", resp.Artifact)

	//ctx := context.Background()
	//res, err := server.Store(ctx, &storeRequest )
	//if err != nil {
	//	t.Error(err.Error())
	//	return
	//}
	//if res.Url == "" || res.Sid == "" {
	//	t.Error("url or sid not valid")
	//	return
	//}
	//server.Delete()

}
