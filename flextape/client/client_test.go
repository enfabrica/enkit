package client

import (
	"context"
	"fmt"
	"testing"

	fpb "github.com/enfabrica/enkit/flextape/proto"
	"github.com/enfabrica/enkit/lib/errdiff"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeClient struct {
	allocateCallCount int
	allocateResponses []*fpb.AllocateResponse
	refreshCallCount int
	refreshResponses []*fpb.RefreshResponse
	refreshCancel func()
}

func (c *fakeClient) Allocate(context.Context, *fpb.AllocateRequest, ...grpc.CallOption) (*fpb.AllocateResponse, error) {
	c.allocateCallCount++
	if c.allocateCallCount-1 < len(c.allocateResponses) {
		return c.allocateResponses[c.allocateCallCount-1], nil
	}
	return nil, fmt.Errorf("no responses left for Allocate()")
}

func (c *fakeClient) Refresh(context.Context, *fpb.RefreshRequest, ...grpc.CallOption) (*fpb.RefreshResponse, error) {
	c.refreshCallCount++
	if len(c.refreshResponses) == 0 {
		c.refreshCancel()
		return nil, fmt.Errorf("no responses")
	}
	if c.refreshCallCount == len(c.refreshResponses) {
		c.refreshCancel()
	}
	return c.refreshResponses[c.refreshCallCount-1], nil
}

func (c *fakeClient) Release(context.Context, *fpb.ReleaseRequest, ...grpc.CallOption) (*fpb.ReleaseResponse, error) {
	return nil, fmt.Errorf("Release() not implemented")
}

func (c *fakeClient) LicensesStatus(context.Context, *fpb.LicensesStatusRequest, ...grpc.CallOption) (*fpb.LicensesStatusResponse, error) {
	return nil, fmt.Errorf("LicensesStatus() not implemented")
}

func TestLicenseClientAcquire(t *testing.T) {
	now := timestamppb.Now()
	testCases := []struct {
		desc              string
		allocateResponses []*fpb.AllocateResponse
		wantCallCount     int
		wantErr           string
	}{
		{
			desc: "allocate immediate success",
			allocateResponses: []*fpb.AllocateResponse{
				&fpb.AllocateResponse{
					ResponseType: &fpb.AllocateResponse_LicenseAllocated{
						LicenseAllocated: &fpb.LicenseAllocated{
							InvocationId:           "a",
							LicenseRefreshDeadline: now,
						},
					},
				},
			},
			wantCallCount: 1,
		},
		{
			desc: "polls while queued",
			allocateResponses: []*fpb.AllocateResponse{
				&fpb.AllocateResponse{
					ResponseType: &fpb.AllocateResponse_Queued{
						Queued: &fpb.Queued{
							InvocationId: "a",
							NextPollTime: now,
						},
					},
				},
				&fpb.AllocateResponse{
					ResponseType: &fpb.AllocateResponse_Queued{
						Queued: &fpb.Queued{
							InvocationId: "a",
							NextPollTime: now,
						},
					},
				},
				&fpb.AllocateResponse{
					ResponseType: &fpb.AllocateResponse_Queued{
						Queued: &fpb.Queued{
							InvocationId: "a",
							NextPollTime: now,
						},
					},
				},
				&fpb.AllocateResponse{
					ResponseType: &fpb.AllocateResponse_LicenseAllocated{
						LicenseAllocated: &fpb.LicenseAllocated{
							InvocationId:           "a",
							LicenseRefreshDeadline: now,
						},
					},
				},
			},
			wantCallCount: 4,
		},
		{
			desc:              "propagates errors",
			allocateResponses: []*fpb.AllocateResponse{},
			wantCallCount:     1,
			wantErr:           "Allocate() failure",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fake := &fakeClient{
				allocateResponses: tc.allocateResponses,
			}
			client := &LicenseClient{
				client: fake,
				invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{
							Vendor:  "xilinx",
							Feature: "foo",
						},
					},
					Owner:    "unittest",
					BuildTag: "test",
				},
			}

			ctx := context.Background()
			gotErr := client.acquire(ctx)

			errdiff.Check(t, gotErr, tc.wantErr)
			assert.Equal(t, tc.wantCallCount, fake.allocateCallCount)
		})
	}
}

func TestLicenseClientRefresh(t *testing.T) {
	now := timestamppb.Now()
	testCases := []struct {
		desc             string
		refreshResponses []*fpb.RefreshResponse
		wantCallCount    int
		wantErr          string
	}{
		{
			desc: "loops until context completed",
			refreshResponses: []*fpb.RefreshResponse{
				&fpb.RefreshResponse{
					InvocationId:           "a",
					LicenseRefreshDeadline: now,
				},
				&fpb.RefreshResponse{
					InvocationId:           "a",
					LicenseRefreshDeadline: now,
				},
				&fpb.RefreshResponse{
					InvocationId:           "a",
					LicenseRefreshDeadline: now,
				},
			},
			wantCallCount: 3,
		},
		{
			desc: "propagates error",
			wantCallCount: 1,
			wantErr: "Refresh() failure",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func (t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			fake := &fakeClient{
				refreshResponses: tc.refreshResponses,
				refreshCancel: cancel,
			}
			client := &LicenseClient{
				client: fake,
				invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{
							Vendor:  "xilinx",
							Feature: "foo",
						},
					},
					Owner:    "unittest",
					BuildTag: "test",
				},
				licenseErr: make(chan error),
			}

			go client.refresh(ctx)
			gotErr := <-client.licenseErr

			errdiff.Check(t, gotErr, tc.wantErr)
			assert.Equal(t, tc.wantCallCount, fake.refreshCallCount)
		})
	}
}
