package astore_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"

	"github.com/enfabrica/enkit/astore/atesting"
	apb "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/astore/server/astore"
	"github.com/enfabrica/enkit/lib/srand"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type AStoreDescriptor struct {
	Connection *grpc.ClientConn
	Server     *astore.Server
}

// RunAStoreServer will spin up an emulated datastore along with an instance of the astore grpc server.
func RunAStoreServer(t *testing.T) (*AStoreDescriptor, atesting.KillAbleProcess) {
	t.Helper()

	killFunctions := atesting.KillAbleProcess{}
	emulatorDescriptor, emulatorKill := atesting.RunEmulatedDatastore(t)
	killFunctions.AddKillable(emulatorKill)

	// Causes the google-could-go/storage library to use a local emulator rather than the real endpoint.
	err := os.Setenv(
		"STORAGE_EMULATOR_HOST",
		fmt.Sprintf("localhost:%d", emulatorDescriptor.Addr.Port),
	)

	buffListener := bufconn.Listen(2048 * 2048)
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return buffListener.Dial()
	}
	grpcServer := grpc.NewServer()
	credsPath, err := runfiles.Rlocation("enkit/astore/testdata/credentials.json")
	require.NoError(t, err)

	credentialString, err := os.ReadFile(credsPath)
	require.NoError(t, err)

	server, err := astore.New(
		rand.New(srand.Source),
		astore.WithCredentialsJSON(credentialString),
		astore.WithSigningJSON(credentialString),
		astore.WithBucket("example-bucket"),
	)
	require.NoError(t, err)

	apb.RegisterAstoreServer(grpcServer, server)

	go func() {
		_ = grpcServer.Serve(buffListener)
	}()

	killGrpcFunc := func() {
		grpcServer.Stop()
	}
	killFunctions.Add(killGrpcFunc)

	conn, err := grpc.DialContext(
		context.Background(),
		"empty",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	return &AStoreDescriptor{
		Connection: conn,
		Server:     server,
	}, killFunctions
}
