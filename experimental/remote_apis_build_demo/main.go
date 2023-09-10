// remote_apis_build_demo demonstrates that we're able to build against:
// * google bytestream protos
// * bazel remote-apis protos
// * buildbarn libraries (bb-storage, bb-remote-execution)
package main

import (
	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-execution/pkg/credentials"
	"github.com/buildbarn/bb-storage/pkg/digest"
	bpb "google.golang.org/genproto/googleapis/bytestream"
)

func main() {
	// Test that it's possible to import a bytestream proto message type
	_ = bpb.NewByteStreamClient(nil)
	// Test that it's possible to import a github.com/bazelbuild/remote-apis proto
	// message type
	_ = repb.BatchReadBlobsRequest{}
	// Test that it's possible to import a type from buildbarn/bb-remote-execution
	_, _, _ = credentials.GetSysProcAttrFromConfiguration(nil)
	// Test that it's possible to import a type from buildbarn/bb-storage
	_ = digest.EmptySet
}
