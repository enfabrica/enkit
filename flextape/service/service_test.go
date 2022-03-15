package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	fpb "github.com/enfabrica/enkit/flextape/proto"
	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// testService returns a preconfigured service, to shorten the testcase
// descriptions.
func testService(initialState state) *Service {
	return testServicePrio(initialState, &FIFOPrioritizer{})
}

func testServicePrio(initialState state, p Prioritizer) *Service {
	return &Service{
		currentState: initialState,
		licenses: map[string]*license{
			"xilinx::feature_foo": &license{
				name:           "xilinx::feature_foo",
				totalAvailable: 2,
				queue:          invocationQueue{},
				allocations:    map[string]*invocation{},
				prioritizer:    p,
			},
		},
		queueRefreshDuration:      5 * time.Second,
		allocationRefreshDuration: 7 * time.Second,
	}
}

// withAllocation is a helper method on a service to set it up with an
// allocation on a particular license.
func (s *Service) withAllocation(licenseType string, inv *invocation) *Service {
	m := s.licenses[licenseType].allocations
	if m == nil {
		m = map[string]*invocation{}
	}
	m[inv.ID] = inv
	s.licenses[licenseType].allocations = m
	return s
}

// withQueued is a helper method on a service to set it up with a queued
// invocation for a license.
func (s *Service) withQueued(licenseType string, inv *invocation) *Service {
	s.licenses[licenseType].queue.Enqueue(inv)
	s.licenses[licenseType].prioritizer.OnEnqueue(inv)
	return s
}

// fakeID serves as a fake unique ID generator for testing purposes.
type fakeID struct {
	counter int64
}

// Generate returns a monotonically increasing ID as a string.
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
		req          *fpb.AllocateRequest
		want         *fpb.AllocateResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc:   "too many licenses",
			server: testService(stateStarting),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
						&fpb.License{Vendor: "xilinx", Feature: "feature_bar"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "",
				},
			},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "exactly one license spec",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations:    map[string]*invocation{},
					prioritizer:    &FIFOPrioritizer{},
				},
			},
		},
		{
			desc:   "unknown license type",
			server: testService(stateStarting),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "unknown_feature"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "",
				},
			},
			wantErrCode: codes.NotFound,
			wantErr:     "unknown license type",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations:    map[string]*invocation{},
					prioritizer:    &FIFOPrioritizer{},
				},
			},
		},
		{
			desc:   "new invocations only enqueued during startup",
			server: testService(stateStarting),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2345",
					Id:       "2",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "2",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 2,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
						&invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_2345", LastCheckin: start, QueueID: 2},
					},
					allocations: map[string]*invocation{},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2345",
					Id:       "2",
				},
			},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not found",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_3",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
						"2": &invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, tc.want, got)
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
		req          *fpb.RefreshRequest
		want         *fpb.RefreshResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc:   "error when invocation_id not set",
			server: testService(stateStarting),
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "invocation_id must be set",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations:    map[string]*invocation{},
					prioritizer:    &FIFOPrioritizer{},
				},
			},
		},
		{
			desc:   "error when multiple licenses specified",
			server: testService(stateStarting),
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "1",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
						&fpb.License{Vendor: "xilinx", Feature: "feature_bar"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "exactly one license spec",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations:    map[string]*invocation{},
					prioritizer:    &FIFOPrioritizer{},
				},
			},
		},
		{
			desc:   "allocates when invocation_id not found during starting state",
			server: testService(stateStarting),
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "1",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			want: &fpb.RefreshResponse{
				InvocationId:           "1",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "5",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1",
				},
			},
			want: &fpb.RefreshResponse{
				InvocationId:           "5",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "1",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			wantErrCode: codes.ResourceExhausted,
			wantErr:     "no available licenses",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
				},
			},
		},
		{
			desc:   "error when invocation_id not found during running state",
			server: testService(stateRunning),
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "1",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not allocated",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations:    map[string]*invocation{},
					prioritizer:    &FIFOPrioritizer{},
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
			req: &fpb.RefreshRequest{
				Invocation: &fpb.Invocation{
					Id: "5",
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1",
				},
			},
			want: &fpb.RefreshResponse{
				InvocationId:           "5",
				LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, tc.want, got)
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
		req          *fpb.ReleaseRequest
		want         *fpb.ReleaseResponse
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
			req:         &fpb.ReleaseRequest{},
			wantErrCode: codes.InvalidArgument,
			wantErr:     "invocation_id must be set",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
			req:  &fpb.ReleaseRequest{InvocationId: "5"},
			want: &fpb.ReleaseResponse{},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
				},
			},
		},
		{
			desc: "unqueues invocations successfully",
			server: testService(stateRunning).withQueued("xilinx::feature_foo", &invocation{
				ID:          "5",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}).withQueued("xilinx::feature_foo", &invocation{
				ID:          "8",
				Owner:       "unit_test",
				BuildTag:    "tag_2",
				LastCheckin: start,
			}),
			req:  &fpb.ReleaseRequest{InvocationId: "5"},
			want: &fpb.ReleaseResponse{},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &FIFOPrioritizer{},
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
			req:         &fpb.ReleaseRequest{InvocationId: "4"},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not found",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, tc.want, got)
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
		req          *fpb.LicensesStatusRequest
		want         *fpb.LicensesStatusResponse
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
			req: &fpb.LicensesStatusRequest{},
			want: &fpb.LicensesStatusResponse{
				LicenseStats: []*fpb.LicenseStats{
					&fpb.LicenseStats{
						License:           &fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
						TotalLicenseCount: 2,
						AllocatedCount:    2,
						QueuedCount:       1,
						AllocatedInvocations: []*fpb.Invocation{
							&fpb.Invocation{
								Id:       "5",
								Owner:    "unit_test",
								BuildTag: "tag_1",
							},
							&fpb.Invocation{
								Id:       "8",
								Owner:    "unit_test",
								BuildTag: "tag_2",
							},
						},
						QueuedInvocations: []*fpb.Invocation{
							&fpb.Invocation{
								Id:       "9",
								Owner:    "unit_test",
								BuildTag: "tag_3",
							},
						},
						Timestamp: timestamppb.New(start),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "9", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, tc.want, got)
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
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start.Add(-10 * time.Second)},
					},
					prioritizer: &FIFOPrioritizer{},
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
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start, QueueID: 1},
						&invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start, QueueID: 2},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"3": &invocation{ID: "3", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start},
					},
					prioritizer: &FIFOPrioritizer{},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}))
		})
	}
}

func TestPrioritizerBasic(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	testCases := []struct {
		desc         string
		server       *Service
		req          *fpb.AllocateRequest
		want         *fpb.AllocateResponse
		wantErrCode  codes.Code
		wantErr      string
		wantLicenses map[string]*license
	}{
		{
			desc:   "new invocations only enqueued during startup",
			server: testServicePrio(stateStarting, NewEvenOwnersPrioritizer()),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{"1": 1}, enqueued: map[string]uint64{"unit_test": 1}, allocated: map[string]uint64{}, dequeued: map[string]uint64{}},
				},
			},
		},
		{
			desc: "returns allocation success when allocated during startup",
			server: testServicePrio(stateStarting, NewEvenOwnersPrioritizer()).withAllocation("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					prioritizer: NewEvenOwnersPrioritizer(),
				},
			},
		},
		{
			desc: "returns queued when invocation already in queue during startup",
			server: testServicePrio(stateStarting, NewEvenOwnersPrioritizer()).withQueued("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{"1": 1}, enqueued: map[string]uint64{"unit_test": 1}, dequeued: map[string]uint64{}, allocated: map[string]uint64{}},
				},
			},
		},
		{
			desc: "returns queued when invocation_id not found during startup",
			server: testServicePrio(stateStarting, NewEvenOwnersPrioritizer()).withQueued("xilinx::feature_foo", &invocation{
				ID:          "1",
				Owner:       "unit_test",
				BuildTag:    "tag_1234",
				LastCheckin: start,
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2345",
					Id:       "2",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "2",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 2,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
						&invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_2345", LastCheckin: start, QueueID: 2},
					},
					allocations: map[string]*invocation{},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{"1": 1, "2": 2}, enqueued: map[string]uint64{"unit_test": 2}, dequeued: map[string]uint64{}, allocated: map[string]uint64{}},
				},
			},
		},
		{
			desc: "returns error when invocation_id not found during running state",
			server: testServicePrio(stateRunning, NewEvenOwnersPrioritizer()).withQueued("xilinx::feature_foo", &invocation{
				ID:          "1",
				Owner:       "unit_test",
				BuildTag:    "tag_1234",
				LastCheckin: start,
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2345",
					Id:       "2",
				},
			},
			wantErrCode: codes.FailedPrecondition,
			wantErr:     "invocation_id not found",
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{"1": 1}, enqueued: map[string]uint64{"unit_test": 1}, dequeued: map[string]uint64{}, allocated: map[string]uint64{}},
				},
			},
		},
		{
			desc: "queues invocation when no license available while running",
			server: testServicePrio(stateRunning, NewEvenOwnersPrioritizer()).withAllocation("xilinx::feature_foo", &invocation{
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
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_3",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_Queued{
					Queued: &fpb.Queued{
						InvocationId:  "1",
						NextPollTime:  timestamppb.New(start.Add(5 * time.Second)),
						QueuePosition: 1,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue: invocationQueue{
						&invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_3", LastCheckin: start, QueueID: 1},
					},
					allocations: map[string]*invocation{
						"5": &invocation{ID: "5", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
						"8": &invocation{ID: "8", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
					},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{"1": 1}, enqueued: map[string]uint64{"unit_test": 1}, dequeued: map[string]uint64{}, allocated: map[string]uint64{}},
				},
			},
		},
		{
			desc: "returns allocation success when allocated during running state",
			server: testServicePrio(stateRunning, NewEvenOwnersPrioritizer()).withAllocation("xilinx::feature_foo", &invocation{
				ID:       "1",
				Owner:    "unit_test",
				BuildTag: "tag_1234",
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_1234",
					Id:       "1",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_1234", LastCheckin: start},
					},
					prioritizer: NewEvenOwnersPrioritizer(),
				},
			},
		},
		{
			desc: "returns allocation success for new request when license available while running",
			server: testServicePrio(stateRunning, NewEvenOwnersPrioritizer()).withAllocation("xilinx::feature_foo", &invocation{
				ID:          "2",
				Owner:       "unit_test",
				BuildTag:    "tag_1",
				LastCheckin: start,
			}),
			req: &fpb.AllocateRequest{
				Invocation: &fpb.Invocation{
					Licenses: []*fpb.License{
						&fpb.License{Vendor: "xilinx", Feature: "feature_foo"},
					},
					Owner:    "unit_test",
					BuildTag: "tag_2",
				},
			},
			want: &fpb.AllocateResponse{
				ResponseType: &fpb.AllocateResponse_LicenseAllocated{
					LicenseAllocated: &fpb.LicenseAllocated{
						InvocationId:           "1",
						LicenseRefreshDeadline: timestamppb.New(start.Add(7 * time.Second)),
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::feature_foo": &license{
					name:           "xilinx::feature_foo",
					totalAvailable: 2,
					queue:          invocationQueue{},
					allocations: map[string]*invocation{
						"1": &invocation{ID: "1", Owner: "unit_test", BuildTag: "tag_2", LastCheckin: start},
						"2": &invocation{ID: "2", Owner: "unit_test", BuildTag: "tag_1", LastCheckin: start},
					},
					prioritizer: &EvenOwnersPrioritizer{position: map[string]uint64{}, enqueued: map[string]uint64{}, dequeued: map[string]uint64{}, allocated: map[string]uint64{"unit_test": 1}},
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

			testutil.AssertCmp(t, tc.server.licenses, tc.wantLicenses, cmp.AllowUnexported(invocation{}, license{}, EvenOwnersPrioritizer{}))
			assert.Equal(t, tc.wantErrCode.String(), status.Code(gotErr).String())
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, tc.want, got)
		})
	}
}

func TestPrioritization(t *testing.T) {
	start := time.Now()
	currentTime := start
	now := &currentTime

	idGen := &fakeID{}
	stubs := gostub.Stub(&generateRandomID, idGen.Generate)
	stubs.Stub(&timeNow, func() time.Time {
		return *now
	})
	defer stubs.Reset()

	server := &Service{
		currentState: stateRunning,
		licenses: licensesFromConfig(&fpb.Config{
			LicenseConfigs: []*fpb.LicenseConfig{&fpb.LicenseConfig{
				Quantity:    4,
				Prioritizer: &fpb.LicenseConfig_EvenOwners{},
				License:     &fpb.License{Vendor: "xilinx", Feature: "foo"},
			}},
		}),
		queueRefreshDuration:      5 * time.Second,
		allocationRefreshDuration: 7 * time.Second,
	}
	ctx := context.Background()

	req := &fpb.AllocateRequest{Invocation: &fpb.Invocation{
		Owner:    "donnie",
		BuildTag: "tag1",
		Licenses: []*fpb.License{&fpb.License{
			Vendor:  "xilinx",
			Feature: "foo",
		}},
	}}

	// Allocate all 4 licenses to one user.
	dqids := []string{}
	for i := 0; i < 4; i++ {
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)
		allocated, converted := resp.ResponseType.(*fpb.AllocateResponse_LicenseAllocated)
		assert.True(t, converted)
		dqids = append(dqids, allocated.LicenseAllocated.InvocationId)
	}

	// Next 4 requests should end up in queue.
	for i := 0; i < 4; i++ {
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)
		queued, converted := resp.ResponseType.(*fpb.AllocateResponse_Queued)
		assert.True(t, converted)
		assert.Equal(t, uint32(i+1), queued.Queued.QueuePosition)
		dqids = append(dqids, queued.Queued.InvocationId)
	}

	// Joe comes by. Look at that: he'll jump ahead in queue!
	req.Invocation.Owner = "joe"
	jqids := []string{}
	for i := 0; i < 3; i++ {
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)
		queued, converted := resp.ResponseType.(*fpb.AllocateResponse_Queued)
		assert.True(t, converted)
		assert.Equal(t, uint32(i+1), queued.Queued.QueuePosition)
		jqids = append(jqids, queued.Queued.InvocationId)
	}

	// Dear donnie at at next poll will find out that his allocations were bumped.
	for ix, id := range dqids[4:] {
		req.Invocation.Id = id
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)

		queued, converted := resp.ResponseType.(*fpb.AllocateResponse_Queued)
		assert.True(t, converted)
		// Note the +3 here, comes from the # of allocations from joe.
		assert.Equal(t, uint32(ix+1+3), queued.Queued.QueuePosition)
	}

	// Now george comes by. He'll get similar priority as joe.
	req.Invocation.Owner = "george"
	req.Invocation.Id = ""
	gqids := []string{}
	for i := 0; i < 2; i++ {
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)
		queued, converted := resp.ResponseType.(*fpb.AllocateResponse_Queued)
		assert.True(t, converted)
		// Stable order: joe and george have "same priority", but joe came first!
		// So george gains second place for each slot after joe.
		assert.Equal(t, uint32((2*i)+2), queued.Queued.QueuePosition)
		gqids = append(gqids, queued.Queued.InvocationId)
	}

	// Donnie now releases 3 licenses, with only 1 now allocated.
	// Joe and George should each get one of the freed slots.
	// But now they're all even, so the last slot could go to any one of them.
	for _, id := range dqids[:3] {
		rel := &fpb.ReleaseRequest{InvocationId: id}
		_, err := server.Release(ctx, rel)
		assert.NoError(t, err)
	}

	server.janitor()
	for _, id := range []string{jqids[0], gqids[0], jqids[1]} {
		req.Invocation.Id = id
		resp, err := server.Allocate(ctx, req)
		assert.Nil(t, err, "error %s", err)

		_, converted := resp.ResponseType.(*fpb.AllocateResponse_LicenseAllocated)
		assert.True(t, converted, "%+v", resp.ResponseType)
	}

	// Donnie releases one more license. Now he is behind the others, with 0
	// allocations. He should go first.
	rel := &fpb.ReleaseRequest{InvocationId: dqids[3]}
	_, err := server.Release(ctx, rel)
	assert.NoError(t, err)
	server.janitor()

	// Let's first check that neither george nor joe will get a license.
	req.Invocation.Id = gqids[1]
	resp, err := server.Allocate(ctx, req)
	assert.Nil(t, err, "error %s", err)
	queued, converted := resp.ResponseType.(*fpb.AllocateResponse_Queued)
	assert.True(t, converted, "%+v", resp.ResponseType)
	assert.Equal(t, uint32(1), queued.Queued.QueuePosition)

	req.Invocation.Id = jqids[2]
	resp, err = server.Allocate(ctx, req)
	assert.Nil(t, err, "error %s", err)
	queued, converted = resp.ResponseType.(*fpb.AllocateResponse_Queued)
	assert.True(t, converted, "%+v", resp.ResponseType)
	// Joe is still at 2 licenses, so comes later.
	assert.Equal(t, uint32(3), queued.Queued.QueuePosition)

	// Donnie will get his license!
	req.Invocation.Id = dqids[4]
	resp, err = server.Allocate(ctx, req)
	assert.Nil(t, err, "error %s", err)
	_, converted = resp.ResponseType.(*fpb.AllocateResponse_LicenseAllocated)
	assert.True(t, converted, "%+v", resp.ResponseType)
}

func TestLicensesFromConfig(t *testing.T) {
	testCases := []struct {
		desc         string
		config       *fpb.Config
		wantLicenses map[string]*license
	}{
		{
			desc: "unspecified prioritizer",
			config: &fpb.Config{
				LicenseConfigs: []*fpb.LicenseConfig{
					&fpb.LicenseConfig{
						License: &fpb.License{
							Vendor:  "xilinx",
							Feature: "foo_tool",
						},
						Quantity: 4,
					},
				},
			},
			wantLicenses: map[string]*license{
				"xilinx::foo_tool": &license{
					name:           "xilinx::foo_tool",
					totalAvailable: 4,
					allocations:    map[string]*invocation{},
					queue:          nil,
					prioritizer:    &FIFOPrioritizer{},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := licensesFromConfig(tc.config)
			assert.Equal(t, tc.wantLicenses, got)
		})
	}
}
