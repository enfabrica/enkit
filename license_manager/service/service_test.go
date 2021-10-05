package service

import (
	"context"
	"testing"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAllocateUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	wantCode := codes.Unimplemented
	req := &lmpb.AllocateRequest{}

	_, err := s.Allocate(ctx, req)
	if gotCode := status.Code(err); gotCode != wantCode {
		t.Errorf("got code %v; want code %v", gotCode, wantCode)
	}
}

func TestRefreshUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	wantCode := codes.Unimplemented
	req := &lmpb.RefreshRequest{}

	_, err := s.Refresh(ctx, req)
	if gotCode := status.Code(err); gotCode != wantCode {
		t.Errorf("got code %v; want code %v", gotCode, wantCode)
	}
}

func TestReleaseUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	wantCode := codes.Unimplemented
	req := &lmpb.ReleaseRequest{}

	_, err := s.Release(ctx, req)
	if gotCode := status.Code(err); gotCode != wantCode {
		t.Errorf("got code %v; want code %v", gotCode, wantCode)
	}
}

func TestLicensesStatusUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	wantCode := codes.Unimplemented
	req := &lmpb.LicensesStatusRequest{}

	_, err := s.LicensesStatus(ctx, req)
	if gotCode := status.Code(err); gotCode != wantCode {
		t.Errorf("got code %v; want code %v", gotCode, wantCode)
	}
}
