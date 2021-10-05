package service

import (
	"context"
	"testing"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAllocateUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	req := &lmpb.AllocateRequest{}

	_, err := s.Allocate(ctx, req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestRefreshUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	req := &lmpb.RefreshRequest{}

	_, err := s.Refresh(ctx, req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestReleaseUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	req := &lmpb.ReleaseRequest{}

	_, err := s.Release(ctx, req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestLicensesStatusUnimplemented(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	req := &lmpb.LicensesStatusRequest{}

	_, err := s.LicensesStatus(ctx, req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}
