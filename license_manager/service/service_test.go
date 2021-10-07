package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/enfabrica/enkit/lib/errdiff"
	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testService(initialState state) *Service {
	return &Service{
		currentState: initialState,
		licenses: map[string]*license{
			"xilinx::feature_foo": &license{
				totalAvailable: 2,
				queue:          []*invocation{},
				allocations:    map[string]*invocation{},
			},
		},
		queueRefreshDuration:      5 * time.Second,
		allocationRefreshDuration: 7 * time.Second,
	}
}

func (s *Service) withAllocation(licenseType string, inv *invocation) *Service {
	m := s.licenses[licenseType].allocations
	if m == nil {
		m = map[string]*invocation{}
	}
	m[inv.ID] = inv
	s.licenses[licenseType].allocations = m
	return s
}

func (s *Service) withQueued(licenseType string, inv *invocation) *Service {
	s.licenses[licenseType].queue = append(s.licenses[licenseType].queue, inv)
	return s
}

func assertProtoEqual(t *testing.T, got proto.Message, want proto.Message) {
	t.Helper()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Proto messages do not match:\n%s\n", diff)
	}
}

func assertCmp(t *testing.T, got interface{}, want interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("Objects are not equal:\n%s\n", diff)
	}
}

type fakeID struct {
	counter int64
}

func (f *fakeID) Generate() (string, error) {
	f.counter++
	return strconv.FormatInt(f.counter, 10), nil
}

// Allocate
//     license type not known -> error NOT FOUND
//     State == STARTUP, no invocation_id -> invocation_id created, invocation enqueued
//     State == STARTUP, invocation_id allocated -> allocation success
//     State == STARTUP, invocation_id queued -> queued
//     State == STARTUP, invocation_id not found -> queued
//   State == RUNNING, no invocation_id, license available -> invocation_id created, invocation allocated
//     State == RUNNING, no invocation_id, no license available -> invocation_id created, invocation queued
//     State == RUNNING, invocation_id allocated -> allocation success
//     State == RUNNING, invocation_id not found -> error
func TestAllocate(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		req          *lmpb.AllocateRequest
		want         *lmpb.AllocateResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc:   "too many licenses",
			server: testService(stateStarting),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					&lmpb.License{Vendor: "xilinx", Feature: "feature_bar"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "",
			},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "exactly one license spec",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations:    map[string]*invocation{},
				},
			},
		},
		{
			desc:   "unknown license type",
			server: testService(stateStarting),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "unknown_feature"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "",
			},
			wantErrCode: codes.NotFound,
			wantErr:     "unknown license type",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations:    map[string]*invocation{},
				},
			},
		},
		{
			desc:   "new invocations only enqueued during startup",
			server: testService(stateStarting),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_Queued{
					Queued: &lmpb.Queued{
						InvocationId: "1",
						NextPollTime: timestamppb.New(start.Add(5 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					allocations: map[string]*invocation{},
				},
			},
		},
		{
			desc: "returns allocation success when allocated during startup",
			server: testService(stateStarting).withAllocation("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "1",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &lmpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "returns queued when invocation already in queue during startup",
			server: testService(stateStarting).withQueued("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "1",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_Queued{
					Queued: &lmpb.Queued{
						InvocationId: "1",
						NextPollTime: timestamppb.New(start.Add(5 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					allocations: map[string]*invocation{},
				},
			},
		},
		{
			desc: "returns queued when invocation_id not found during startup",
			server: testService(stateStarting).withQueued("xilinx::feature_foo", &invocation{
				ID:          "1",
				Owner:       "unit_test",
				BuildTag:    "tag_1234",
				LastCheckin: start,
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_2345",
				InvocationId: "2",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_Queued{
					Queued: &lmpb.Queued{
						InvocationId: "2",
						NextPollTime: timestamppb.New(start.Add(5 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
						&invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_2345", LastCheckin: start},
					},
					allocations: map[string]*invocation{},
				},
			},
		},
		{
			desc: "returns error when invocation_id not found during running state",
			server: testService(stateRunning).withQueued("xilinx::feature_foo", &invocation{
				ID:          "1",
				Owner:       "unit_test",
				BuildTag:    "tag_1234",
				LastCheckin: start,
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_2345",
				InvocationId: "2",
			},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not found",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					allocations: map[string]*invocation{},
				},
			},
		},
		{
			desc: "queues invocation when no license available while running",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "8",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start,
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_3",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_Queued{
					Queued: &lmpb.Queued{
						InvocationId: "1",
						NextPollTime: timestamppb.New(start.Add(5 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "returns allocation success when allocated during running state",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:        "unit_test",
				BuildTag:     "tag_1234",
				InvocationId: "1",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &lmpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "returns allocation success for new request when license available while running",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "2",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}),
			req: &lmpb.AllocateRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
			},
			want: &lmpb.AllocateResponse{
				ResponseType: &lmpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &lmpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
						"2": &invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			idGen := &fakeID{}
			stubs := gostub.Stub(&generateRandomID, idGen.Generate)
			stubs.Stub(&timeNow, func() time.Time {
				return *now
			})
			defer stubs.Reset()

			got, gotErr := tc.server.Allocate(ctx, tc.req)

			assertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assertProtoEqual(t, tc.want, got)
		})
	}
}

//
// Refresh
//     State == STARTUP, no invocation_id -> error
//   State == STARTUP, invocation_id allocated -> refresh success
//   State == STARTUP, invocation_id not allocated, license available -> allocate and refresh success
//   State == STARTUP, invocation_id not allocated, license not available -> error
//   State == RUNNING, no invocation_id -> error
//   State == RUNNING, invocation_id allocated -> refresh success
//   State == RUNNING, invocation_id not allocated -> error
//
// Release
//   no invocation_id -> error
//   invocation_id allocated -> deallocate successfully
//   invocation_id not allocated -> error
//
// janitor
//   expires stale queued license requests
//   expires stale allocations
//   promotes queued license requests
//   expires stale allocations and promotes queued license requests
//   removes stale "recently expired" allocations

func TestRefresh(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		req          *lmpb.RefreshRequest
		want         *lmpb.RefreshResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc:        "error when invocation_id not set",
			server:      testService(stateStarting),
			req:         &lmpb.RefreshRequest{},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "invocation_id must be set",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations:    map[string]*invocation{},
				},
			},
		},
		{
			desc:   "error when invocation_id not found during running state",
			server: testService(stateRunning),
			req: &lmpb.RefreshRequest{
				InvocationId: "1",
			},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not allocated",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations:    map[string]*invocation{},
				},
			},
		},
		// testcases
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			idGen := &fakeID{}
			stubs := gostub.Stub(&generateRandomID, idGen.Generate)
			stubs.Stub(&timeNow, func() time.Time {
				return *now
			})
			defer stubs.Reset()

			got, gotErr := tc.server.Refresh(ctx, tc.req)

			assertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assertProtoEqual(t, tc.want, got)
		})
	}
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
