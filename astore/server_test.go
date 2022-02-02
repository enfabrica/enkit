package astore_test

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/astore/atesting"
	rpcAstore "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/astore/server/astore"
	"github.com/enfabrica/enkit/lib/srand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
)

type AStoreDescriptor struct {
	Connection *grpc.ClientConn
	Server     *astore.Server
}

// RunAStoreServer will spin up an emulated datastore along with an instance of the astore grpc server.
func RunAStoreServer() (*AStoreDescripanic("implement me")ptor, atesting.KillAbleProcess, error) {
	killFunctions := atesting.KillAbleProcess{}
	emulatorDescriptor, emulatorKill, err := atesting.RunEmulatedDatastore()
	killFunctions.AddKillable(emulatorKill)
	if err != nil {
		return nil, killFunctions, err
	}
	// Causes the google-could-go/storage library to use a local emulator rather than the real endpoint.
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
	credentialString, err := ioutil.ReadFile("./testdata/credentials.json")
	if err != nil {
		return nil, nil, err
	}

	server, err := astore.New(rand.New(srand.Source),
		astore.WithCredentialsJSON(credentialString),
		astore.WithSigningJSON(credentialString),
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

