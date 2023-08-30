package cas

import (
	"context"

	rpb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bspb "github.com/enfabrica/enkit/googleapis/bytestream"
)

type Service struct {
}

func New() *Service {
	return &Service{}
}

func (s *Service) FindMissingBlobs(ctx context.Context, req *rpb.FindMissingBlobsRequest) (*rpb.FindMissingBlobsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *Service) BatchUpdateBlobs(ctx context.Context, req *rpb.BatchUpdateBlobsRequest) (*rpb.BatchUpdateBlobsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *Service) BatchReadBlobs(ctx context.Context, req *rpb.BatchReadBlobsRequest) (*rpb.BatchReadBlobsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *Service) GetTree(req *rpb.GetTreeRequest, stream rpb.ContentAddressableStorage_GetTreeServer) error {
	return status.Error(codes.Unimplemented, "")
}

func (s *Service) Read(req *bspb.ReadRequest, stream bspb.ByteStream_ReadServer) error {
	return status.Error(codes.Unimplemented, "")
}

func (s *Service) Write(req *bspb.WriteRequest, stream bspb.ByteStream_WriteServer) error {
	return status.Error(codes.Unimplemented, "")
}

func (s *Service) QueryWriteStatus(ctx context.Context, req *bspb.QueryWriteStatusRequest) (*bspb.QueryWriteStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
