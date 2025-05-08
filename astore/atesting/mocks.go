//go:build !release
// +build !release

package atesting

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func RunEmulatedDatastore(t *testing.T) (*EmulatedDatastoreDescriptor, KillAbleProcess) {
	t.Helper()

	testTmpdir := os.Getenv("TEST_TMPDIR")
	// There's currently no easy way to get an unallocated port number without
	// opening said port; hardcode a port here (requires that tests that use
	// this not run concurrently with each other)
	testAddr, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:8432")
	require.NoError(t, err)

	t.Logf("Starting emulated Datastore on address %q", testAddr)
	cmd := exec.Command(
		"gcloud",
		"beta",
		"emulators",
		"datastore",
		"start",
		"--no-store-on-disk",
		"--data-dir="+testTmpdir,
		"--host-port="+testAddr.String(),
		"--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	outputStdErrPipe, err := cmd.StderrPipe()
	require.NoError(t, err)

	err = cmd.Start()
	killFunc := []func(){
		func() {
			fmt.Println("killing emulator datastore")
			if err := cmd.Process.Kill(); err != nil {
				log.Fatalln(fmt.Sprintf("error killing process %d", cmd.Process.Pid))
			}
		},
	}
	require.NoError(t, err)

	datastoreBooted := make(chan bool)
	// TODO(adam): concatenate stdout and stderr?
	// the datastore emulator writes all logs to the error channel for some reason
	scannerErr := bufio.NewScanner(outputStdErrPipe)
	emulatorOutputText := ""
	go func() {
		for scannerErr.Scan() {
			emulatorOutputText += scannerErr.Text()
			if strings.Contains(scannerErr.Text(), "Dev App Server is now running") {
				t.Logf("Started emulated Datastore successfully")
				datastoreBooted <- true
			}
		}
	}()
	select {
	case <-time.After(15 * time.Second):
		require.FailNowf(t, "timeout on starting the emulator", "output is %v", emulatorOutputText)
	case result := <-datastoreBooted:
		if result {
			return &EmulatedDatastoreDescriptor{
				Addr: testAddr,
			}, killFunc
		}
		require.FailNowf(t, "unable to start emulator", "output is %v", emulatorOutputText)
	}
	require.FailNow(t, "unreachable")
	return nil, nil
}

type KillAbleProcess []func()

func (k *KillAbleProcess) KillAll() {
	fmt.Println("running kill functions")
	for _, killFunc := range *k {
		fmt.Println("running kill function")
		killFunc()
	}
}

func (k *KillAbleProcess) AddKillable(process KillAbleProcess) {
	*k = append(*k, process...)
}

func (k *KillAbleProcess) Add(p func()) {
	*k = append(*k, p)
}
