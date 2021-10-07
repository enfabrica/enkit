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
			desc:   "error when invocation_id not set",
			server: testService(stateStarting),
			req: &lmpb.RefreshRequest{
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
			},
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
			desc:   "error when multiple licenses specified",
			server: testService(stateStarting),
			req: &lmpb.RefreshRequest{
				InvocationId: "1",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					&lmpb.License{Vendor: "xilinx", Feature: "feature_bar"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
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
			desc:   "allocates when invocation_id not found during starting state",
			server: testService(stateStarting),
			req: &lmpb.RefreshRequest{
				InvocationId: "1",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
			},
			want: &lmpb.RefreshResponse{
				InvocationId:           "1",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "refreshes during starting state",
			server: testService(stateStarting).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}),
			req: &lmpb.RefreshRequest{
				InvocationId: "5",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_1",
			},
			want: &lmpb.RefreshResponse{
				InvocationId:           "5",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "error when invocation_id not found and no license available during starting state",
			server: testService(stateStarting).withAllocation("xilinx::feature_foo", &invocation{
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
			req: &lmpb.RefreshRequest{
				InvocationId: "1",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
			},
			wantErrCode: codes.ResourceExhausted,
			wantErr:     "no available licenses",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
				},
			},
		},
		{
			desc:   "error when invocation_id not found during running state",
			server: testService(stateRunning),
			req: &lmpb.RefreshRequest{
				InvocationId: "1",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_2",
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
		{
			desc: "refreshes allocation during running state",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}),
			req: &lmpb.RefreshRequest{
				InvocationId: "5",
				Licenses: []*lmpb.License{
					&lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
				},
				Owner:    "unit_test",
				BuildTag: "tag_1",
			},
			want: &lmpb.RefreshResponse{
				InvocationId:           "5",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
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

func TestRelease(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		req          *lmpb.ReleaseRequest
		want         *lmpb.ReleaseResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc: "error when invocation_id not set",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}),
			req:         &lmpb.ReleaseRequest{},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "invocation_id must be set",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "deallocates all licenses successfully",
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
			req:  &lmpb.ReleaseRequest{InvocationId: "5"},
			want: &lmpb.ReleaseResponse{},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "errors when allocation not recognized",
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
			req:         &lmpb.ReleaseRequest{InvocationId: "4"},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not found",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
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

			got, gotErr := tc.server.Release(ctx, tc.req)

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

func TestLicensesStatus(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		req          *lmpb.LicensesStatusRequest
		want         *lmpb.LicensesStatusResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc: "returns licenses status",
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
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "9",
				Owner:       "unit_test",
				BuildTag:    "tag_3",
				LastCheckin: start,
			}),
			req: &lmpb.LicensesStatusRequest{},
			want: &lmpb.LicensesStatusResponse{
				LicenseStats: []*lmpb.LicenseStats{
					&lmpb.LicenseStats{
						License:           &lmpb.License{Vendor: "xilinx", Feature: "feature_foo"},
						TotalLicenseCount: 2,
						AllocatedCount:    2,
						QueuedCount:       1,
						Timestamp:         timestamppb.New(start),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "9", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
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

			got, gotErr := tc.server.LicensesStatus(ctx, tc.req)

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

func TestJanitor(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		endTime      time.Time
		wantLicenses map[string]*license
	}{
		{
			desc: "does nothing during starting state",
			server: testService(stateStarting).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "8",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start.Add(-10 * time.Second), // stale
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "3",
				Owner:       "unit_test",
				BuildTag:    "tag_3",
				LastCheckin: start,
			}),
			endTime: start,
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start.Add(-10 * time.Second)},
					},
				},
			},
		},
		{
			desc: "expires stale queued license requests",
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
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "1",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "2",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start.Add(-10 * time.Second), // stale
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "3",
				Owner:       "unit_test",
				BuildTag:    "tag_3",
				LastCheckin: start,
			}),
			endTime: start,
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue: []*invocation{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						&invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "expires stale allocations",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "8",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start.Add(-10 * time.Second), // stale
			}),
			endTime: start,
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
				},
			},
		},
		{
			desc: "promotes queued license requests",
			server: testService(stateRunning).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "8",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start.Add(-10 * time.Second), // stale
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "3",
				Owner:       "unit_test",
				BuildTag:    "tag_3",
				LastCheckin: start,
			}),
			endTime: start,
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					totalAvailable: 2,
					queue:          []*invocation{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"3": &invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			idGen := &fakeID{}
			stubs := gostub.Stub(&generateRandomID, idGen.Generate)
			stubs.Stub(&timeNow, func() time.Time {
				return *now
			})
			defer stubs.Reset()

			*now = tc.endTime
			tc.server.janitor()

			assertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
		})
	}
}
