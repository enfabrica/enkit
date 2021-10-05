package service

import (
	"context"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct{}

func (s *Service) Allocate(ctx context.Context, req *lmpb.AllocateRequest) (*lmpb.AllocateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Allocate() is not yet implemented")
}

func (s *Service) Refresh(ctx context.Context, req *lmpb.RefreshRequest) (*lmpb.RefreshResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Refresh() is not yet implemented")
}

func (s *Service) Release(ctx context.Context, req *lmpb.ReleaseRequest) (*lmpb.ReleaseResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Release() is not yet implemented")
}

func (s *Service) LicensesStatus(ctx context.Context, req *lmpb.LicensesStatusRequest) (*lmpb.LicensesStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "LicensesStatus() is not yet implemented")
}
