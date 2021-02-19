// +build !release

package e2e

import (
	"context"
	"fmt"
	astore2 "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/astore/server/astore"
	"github.com/enfabrica/enkit/lib/srand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
)

type MinioDescriptor struct {
	Port uint16
}

//RunMinioServer will spin up a minio serer using the local docker daemon
//it also returns a func to call that will close and destroy the running image
//the port and network bind are determined by docker and returned
func RunMinioServer() (MinioDescriptor, func() error, error) {
	_ = "gcloud beta emulators datastore start --no-store-on-disk"
	return MinioDescriptor{}, nil, nil
}

type AStoreDescriptor struct {
	Connection *grpc.ClientConn
	Server     *astore.Server
}

//RunMinioServer will spin up a minio serer using the local docker daemon
//it also returns a func to call that will close and destroy the running image
//the port and network bind are determined by docker and returned
func RunAStoreServer() (AStoreDescriptor, func(), error) {
	err := os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")
	if err != nil {
		return AStoreDescriptor{}, nil, err
	}
	cmd := exec.Command("gcloud beta emulators datastore start --no-store-on-disk")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	doneChannel := make(chan bool)
	go func() {
		err := cmd.Run()
		if err != nil {
			log.Fatal("could not run emulator")
		}
		for {
			b, err := cmd.Output()
			if err != nil {
				log.Fatal(err.Error())
			}
			println("output: " + string(b))
			stringOutput := string(b)
			select {
			case x, ok := <- doneChannel:
				if ok {
					fmt.Printf("Value %d was read.\n", x)
					continue
				} else {
					fmt.Println("Channel closed!")
				}
			default:
				if strings.Contains(stringOutput, "Dev App Server is now running.") {
					doneChannel <- true
				}
			}

		}
	}()
	<- doneChannel
	err = os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9000")
	if err != nil {
		return AStoreDescriptor{}, nil, err
	}

	os.Getenv("STORAGE_EMULATOR_HOST")
	buffListener := bufconn.Listen(2048 * 2048)
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return buffListener.Dial()
	}
	server, err := astore.New(rand.New(srand.Source),
		astore.WithCredentialsJSON([]byte(``)),
		astore.WithSigningJSON([]byte(``)),
		astore.WithBucket("example-bucket"))

	if err != nil {
		return AStoreDescriptor{}, nil, err
	}
	grpcServer := grpc.NewServer()
	astore2.RegisterAstoreServer(grpcServer, server)
	//authServer, err := auth2.New(
	//	rand.New(srand.Source),
	//	auth2.WithAuthURL("http://empty"))
	//auth.RegisterAuthServer(grpcServer, authServer)
	conn, err := grpc.DialContext(context.Background(),
		"empty", grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure())

	a := func() {
		cmd.Process.Kill()
		if err := grpcServer.Serve(buffListener); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}
	return AStoreDescriptor{
		Connection: conn,
		Server: server,
	}, a, nil
}
