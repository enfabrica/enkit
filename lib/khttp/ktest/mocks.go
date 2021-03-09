// +build !release

package ktest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	rpcAstore "github.com/enfabrica/enkit/astore/rpc/astore"
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
	"syscall"
	"time"
)

type MinioDescriptor struct {
	Port uint16
}

// RunMinioServer will spin up a minio serer using the local docker daemon
// it also returns a func to call that will close and destroy the running image
// the port and network bind are determined by docker and returned
func RunMinioServer() (MinioDescriptor, func() error, error) {
	return MinioDescriptor{}, nil, nil
}

type EmulatedDatastoreDescriptor struct {
	Addr *net.TCPAddr
}

func RunEmulatedDatastore() (*EmulatedDatastoreDescriptor, KillAbleProcess, error) {
	emulatorAddr, err := AllocatePort()
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command("gcloud",
		"beta", "emulators", "datastore", "start",
		"--no-store-on-disk",
		fmt.Sprintf("--host-port=127.0.0.1:%d", emulatorAddr.Port),
		"--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	outputStdErrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	err = cmd.Start()
	killFunc := []func(){
		func() {
			fmt.Println("killing emulator datastore")
			if err := cmd.Process.Kill(); err != nil {
				log.Fatalln(fmt.Sprintf("error killing process %d", cmd.Process.Pid))
			}
		}}
	if err != nil {
		return nil, killFunc, err
	}
	datastoreBooted := make(chan bool)
	// TODO(adam): concatenate stdout and stderr?
	// the datastore emulator writes all logs to the error channel for some reason
	scannerErr := bufio.NewScanner(outputStdErrPipe)
	emulatorOutputText := ""
	go func() {
		for scannerErr.Scan() {
			emulatorOutputText += scannerErr.Text()
			if strings.Contains(scannerErr.Text(), "Dev App Server is now running") {
				datastoreBooted <- true
			}
		}
	}()
	select {
	case <-time.After(30 * time.Second):
		return nil, nil, nil
	case result := <-datastoreBooted:
		if result {
			return &EmulatedDatastoreDescriptor{
				Addr: emulatorAddr,
			}, killFunc, nil
		}
		return nil, killFunc, errors.New(fmt.Sprintf("unable to start emulator, output is %v", emulatorOutputText))
	}
}

type AStoreDescriptor struct {
	Connection *grpc.ClientConn
	Server     *astore.Server
}

// RunAStoreServer will spin up an emulated datastore along with an instance of the astore grpc server.
func RunAStoreServer() (*AStoreDescriptor, KillAbleProcess, error) {

	killFunctions := KillAbleProcess{}
	emulatorDescriptor, emulatorKill, err := RunEmulatedDatastore()
	killFunctions.AddKillable(emulatorKill)
	if err != nil {
		return nil, killFunctions, err
	}
	err = os.Setenv(
		"STORAGE_EMULATOR_HOST",
		fmt.Sprintf("localhost:%d", emulatorDescriptor.Addr.Port))
	if err != nil {
		return nil, killFunctions, err
	}
	buffListener := bufconn.Listen(2048 * 2048)
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return buffListener.Dial()
	}
	grpcServer := grpc.NewServer()
	credentialString, err := CheckCredentials()
	if err != nil {
		return nil, nil, err
	}

	server, err := astore.New(rand.New(srand.Source),
		astore.WithCredentialsJSON([]byte(credentialString)),
		astore.WithSigningJSON([]byte(credentialString)),
		astore.WithBucket("example-bucket"))

	if err != nil {
		return nil, killFunctions, err
	}
	rpcAstore.RegisterAstoreServer(grpcServer, server)
	if err := grpcServer.Serve(buffListener); err != nil {
		return nil, killFunctions, err
	}
	killGrpcFunc := func() {
		grpcServer.Stop()
	}
	killFunctions.Add(killGrpcFunc)

	conn, err := grpc.DialContext(context.Background(),
		"empty", grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure())
	if err != nil {
		return nil, killFunctions, err
	}
	return &AStoreDescriptor{
		Connection: conn,
		Server:     server,
	}, killFunctions, nil
}
