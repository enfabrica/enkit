package cas

import (
	"context"
	"testing"

	rpb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"
	bspb "github.com/enfabrica/enkit/googleapis/bytestream"
)

func TestService_FindMissingBlobs(t *testing.T) {
	var want *rpb.FindMissingBlobsResponse = nil
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	got, gotErr := s.FindMissingBlobs(context.Background(), &rpb.FindMissingBlobsRequest{})

	errdiff.Check(t, gotErr, wantErr.Error())
	testutil.AssertCmp(t, got, want)
}

func TestService_BatchUpdateBlobs(t *testing.T) {
	var want *rpb.BatchUpdateBlobsResponse = nil
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	got, gotErr := s.BatchUpdateBlobs(context.Background(), &rpb.BatchUpdateBlobsRequest{})

	errdiff.Check(t, gotErr, wantErr.Error())
	testutil.AssertCmp(t, got, want)
}

func TestService_BatchReadBlobs(t *testing.T) {
	var want *rpb.BatchReadBlobsResponse = nil
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	got, gotErr := s.BatchReadBlobs(context.Background(), &rpb.BatchReadBlobsRequest{})

	errdiff.Check(t, gotErr, wantErr.Error())
	testutil.AssertCmp(t, got, want)
}

func TestService_GetTree(t *testing.T) {
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	gotErr := s.GetTree(&rpb.GetTreeRequest{}, nil)

	errdiff.Check(t, gotErr, wantErr.Error())
}

func TestService_Read(t *testing.T) {
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	gotErr := s.Read(&bspb.ReadRequest{}, nil)

	errdiff.Check(t, gotErr, wantErr.Error())
}

func TestService_Write(t *testing.T) {
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	gotErr := s.Write(&bspb.WriteRequest{}, nil)

	errdiff.Check(t, gotErr, wantErr.Error())
}

func TestService_QueryWriteStatus(t *testing.T) {
	var want *bspb.QueryWriteStatusResponse = nil
	wantErr := status.Error(codes.Unimplemented, "")

	s := &Service{}

	got, gotErr := s.QueryWriteStatus(context.Background(), &bspb.QueryWriteStatusRequest{})

	errdiff.Check(t, gotErr, wantErr.Error())
	testutil.AssertCmp(t, got, want)
}
