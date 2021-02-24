package astore

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/client/astore"
	rpcAstore "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/progress"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"syscall"
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
	astoreDescriptor, end, err := RunAStoreServer()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer end()
	//running this as test ping feature
	client := astore.New(astoreDescriptor.Connection)
	res, _, err := client.List("/test", astore.ListOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Printf("list response is +%v \n", res)
	b, err := ioutil.ReadFile("./testdata/example.yaml")
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
	storeResponse, err := astoreDescriptor.Server.Store(context.Background(), &rpcAstore.StoreRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if storeResponse.GetSid() == "" || storeResponse.GetUrl() == "" {
		t.Fatal(errors.New("invalid store response"))
	}
	resp, err := astoreDescriptor.Server.Commit(context.Background(), &rpcAstore.CommitRequest{
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

func CheckCredentials() (string, error) {
	b, err := ioutil.ReadFile("credentials/creds.json")
	if err != nil {
		return string(b), err
	}
	return string(b), err
}

//RunMinioServer will spin up a minio serer using the local docker daemon
//it also returns a func to call that will close and destroy the running image
//the port and network bind are determined by docker and returned
func RunAStoreServer() (AStoreDescriptor, func(), error) {
	y := exec.Command("echo", "$PATH")
	y.Stdout = os.Stdout
	err := y.Run()
	if err != nil {
		log.Println(err.Error())
	} else {
		p, _ := y.Output()
		fmt.Println("path is ", string(p))
	}
	fmt.Println(os.Getenv("PATH"))
	err = os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")
	if err != nil {
		return AStoreDescriptor{}, nil, err
	}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalln(err)
	}
	allocatedDatastorePort := listener.Addr().(*net.TCPAddr).Port
	cmd := exec.Command("gcloud",
		"beta", "emulators", "datastore", "start",
		"--no-store-on-disk",
		fmt.Sprintf("--host-port=127.0.0.1:%d", allocatedDatastorePort),
		"--quiet")

	//cmd.Stdout = os.Stdout
	//cmd.Stdin = os.Stdin
	//cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	//doneChannel := make(chan bool)
	go func() {
		//err := cmd.Run()
		//if err != nil {
		//	log.Fatalln("could not run emulator", err.Error())
		//}
		//fmt.Println("hello world")
		//for {
		//	b, err := cmd.Output()
		//	if err != nil {
		//		log.Fatal(err.Error())
		//	}
		//	println("output: " + string(b))
		//	stringOutput := string(b)
		//	select {
		//	case x, ok := <-doneChannel:
		//		if ok {
		//			fmt.Printf("Value %d was read.\n", x)
		//			doneChannel <- true
		//			continue
		//		} else {
		//			fmt.Println("Channel closed!")
		//			doneChannel <- true
		//		}
		//	default:
		//		if strings.Contains(stringOutput, "Dev App Server is now running.") {
		//			doneChannel <- true
		//		}
		//	}
		//}
	}()
	outputPipe, err := cmd.StdoutPipe()
	go io.Copy(os.Stdout, outputPipe)
	if err != nil {
		fmt.Println(err.Error())
	}
	err = cmd.Start()
	if err != nil {
		log.Fatalln(err.Error())
	}
	scanner := bufio.NewScanner(outputPipe)
	if scanner.Sc  an() {
		fmt.Println(scanner.Text())
	}
	//
	//err = os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9000")
	//if err != nil {
	//	return AStoreDescriptor{}, nil, err
	//}
	//
	//os.Getenv("STORAGE_EMULATOR_HOST")
	//buffListener := bufconn.Listen(2048 * 2048)
	//bufDialer := func(context.Context, string) (net.Conn, error) {
	//	return buffListener.Dial()
	//}
	//server, err := New(rand.New(srand.Source),
	//	WithCredentialsJSON([]byte(``)),
	//	WithSigningJSON([]byte(``)),
	//	WithBucket("example-bucket"))
	//
	//if err != nil {
	//	return AStoreDescriptor{}, nil, err
	//}
	//grpcServer := grpc.NewServer()
	//rpcAstore.RegisterAstoreServer(grpcServer, server)
	////authServer, err := auth2.New(
	////	rand.New(srand.Source),
	////	auth2.WithAuthURL("http://empty"))
	////auth.RegisterAuthServer(grpcServer, authServer)
	//conn, err := grpc.DialContext(context.Background(),
	//	"empty", grpc.WithContextDialer(bufDialer),
	//	grpc.WithInsecure())
	//
	//a := func() {
	//	cmd.Process.Kill()
	//	if err := grpcServer.Serve(buffListener); err != nil {
	//		log.Fatalf("Server exited with error: %v", err)
	//	}
	//}
	return AStoreDescriptor{
	}, nil, nil
}

type AStoreDescriptor struct {
	Connection *grpc.ClientConn
	Server     *Server
}
