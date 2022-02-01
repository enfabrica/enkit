package kbuildbarn_test

import (
	"bytes"
	"context"
	"github.com/enfabrica/enkit/lib/bes"
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	bbpb "github.com/enfabrica/enkit/third_party/buildbuddy/proto"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"testing"
)

func TestMergeResults(t *testing.T) {
	sameFiles := generateManyFiles(20)
	testResult := &bespb.TestResult{TestActionOutput: sameFiles}
	namedSetOfFiles := &bespb.NamedSetOfFiles{Files: sameFiles}
	e := &bbpb.GetInvocationResponse{
		Invocation: []*bbpb.Invocation{
			{
				InvocationId: "invocation",
				Event: []*bbpb.InvocationEvent{
					{
						BuildEvent: &bespb.BuildEvent{
							Payload: &bespb.BuildEvent_TestResult{TestResult: testResult},
						},
					},
					{
						BuildEvent: &bespb.BuildEvent{
							Payload: &bespb.BuildEvent_NamedSetOfFiles{NamedSetOfFiles: namedSetOfFiles},
						},
					},
				},
			},
		}}
	testHttpClient := newTestHttpClient(t, 200, e)
	buddy := bes.NewTestClient(testHttpClient)
	ctx := context.TODO()
	onlyTestResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithTestResults())
	assert.NoError(t, err)
	assert.Equal(t, len(sameFiles), len(onlyTestResults))
	onlyNamedFileResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithNamedSetOfFiles())
	assert.NoError(t, err)
	assert.Equal(t, len(sameFiles), len(onlyNamedFileResults))
	allResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithNamedSetOfFiles(), kbuildbarn.WithTestResults())
	assert.NoError(t, err)
	assert.Equal(t, len(sameFiles), len(allResults))

	assert.ElementsMatch(t, linkToDestArray(allResults), linkToDestArray(onlyTestResults), linkToDestArray(onlyNamedFileResults))

	testUnique(t, linkToDestArray(allResults))
	testUnique(t, linkToDestArray(onlyNamedFileResults))
	testUnique(t, linkToDestArray(onlyTestResults))

}

func linkToDestArray(l kbuildbarn.HardlinkList) []string {
	var s []string
	for _, v := range l {
		s = append(s, v.Dest)
	}
	return s
}

func TestUniqueResponses(t *testing.T) {
	namedSetSize := 50
	sharedFileSize := 25
	sharedfiles := generateManyFiles(sharedFileSize)
	e := &bbpb.GetInvocationResponse{
		Invocation: []*bbpb.Invocation{
			{
				Event: []*bbpb.InvocationEvent{
					{
						BuildEvent: &bespb.BuildEvent{
							Payload: &bespb.BuildEvent_TestResult{TestResult: &bespb.TestResult{TestActionOutput: sharedfiles}},
						},
					},
					{
						BuildEvent: &bespb.BuildEvent{
							Payload: &bespb.BuildEvent_NamedSetOfFiles{NamedSetOfFiles: &bespb.NamedSetOfFiles{Files: generateManyFiles(namedSetSize)}},
						},
					},
					{
						BuildEvent: &bespb.BuildEvent{
							Payload: &bespb.BuildEvent_NamedSetOfFiles{NamedSetOfFiles: &bespb.NamedSetOfFiles{Files: sharedfiles}},
						},
					},
				},
			},
		}}
	testHttpClient := newTestHttpClient(t, 200, e)
	buddy := bes.NewTestClient(testHttpClient)
	ctx := context.TODO()
	onlyTestResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithTestResults())
	assert.NoError(t, err)
	assert.Equal(t, sharedFileSize, len(onlyTestResults))
	onlyNamedFileResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithNamedSetOfFiles())
	assert.NoError(t, err)
	assert.Equal(t, namedSetSize+sharedFileSize, len(onlyNamedFileResults))
	allResults, err := kbuildbarn.GenerateSymlinks(ctx, buddy, "/base", "invocation", "cluster", kbuildbarn.WithNamedSetOfFiles(), kbuildbarn.WithTestResults())
	assert.NoError(t, err)
	assert.Equal(t, namedSetSize+sharedFileSize, len(allResults))

	testUnique(t, linkToDestArray(allResults))
	testUnique(t, linkToDestArray(onlyNamedFileResults))
	testUnique(t, linkToDestArray(onlyTestResults))
}

type testHttpClient struct {
	cannedResponse *http.Response
	msg            []byte
	code           int
}

func newTestHttpClient(t *testing.T, code int, res proto.Message) *testHttpClient {
	t.Helper()
	if code == 0 {
		code = 200
	}
	msg, err := proto.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal proto: %v", err)
	}
	return &testHttpClient{
		msg:  msg,
		code: code,
	}
}

func (c *testHttpClient) Do(_ *http.Request) (*http.Response, error) {
	cpyMsg := make([]byte, len(c.msg))
	copy(cpyMsg, c.msg)
	return &http.Response{
		Body:       io.NopCloser(bytes.NewBuffer(cpyMsg)),
		StatusCode: c.code,
	}, nil
}

func generateManyFiles(size int) []*bespb.File {
	var toReturn []*bespb.File
	for i := 0; i < size; i++ {
		e := &bespb.File{
			Digest: strconv.Itoa(rand.Int()),
			Length: int64(rand.Int()),
			Name:   strconv.Itoa(rand.Int()),
		}
		toReturn = append(toReturn, e)
	}
	return toReturn
}

func testUnique(t *testing.T, strSlice []string) {
	keys := make(map[string]bool)
	for _, entry := range strSlice {
		if entry == "" {
			t.Error("entry should not be empty string")
		}
		if _, value := keys[entry]; !value {
			keys[entry] = true
		} else {
			assert.Failf(t, "elements we not unique in array, duplicates of %s were found", entry)
		}
	}
}
