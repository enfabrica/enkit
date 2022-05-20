package bes

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	bbpb "github.com/enfabrica/enkit/third_party/buildbuddy/proto"

	"github.com/golang/protobuf/proto"
)

type testHttpClient struct {
	cannedResponse *http.Response
}

func newTestHttpClient(t *testing.T, code int, res proto.Message) *testHttpClient {
	t.Helper()
	if code == 0 {
		code = 200
	}
	msg, err := proto.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal proto: %w", err)
	}
	b := bytes.NewBuffer(msg)
	return &testHttpClient{
		cannedResponse: &http.Response{
			Body:       io.NopCloser(b),
			StatusCode: code,
		},
	}
}

func (c *testHttpClient) Do(req *http.Request) (*http.Response, error) {
	return c.cannedResponse, nil
}

func TestGetBuildEvents(t *testing.T) {
	testCases := []struct {
		desc         string
		invocationID string
		resCode      int
		response     *bbpb.GetInvocationResponse
		wantEvents   []*bespb.BuildEvent
		wantErr      string
	}{
		{
			desc:         "one invocation returned",
			invocationID: "180c8fc1-bfe1-444e-a00c-2d53768125b0",
			response: &bbpb.GetInvocationResponse{
				Invocation: []*bbpb.Invocation{
					&bbpb.Invocation{
						Event: []*bbpb.InvocationEvent{
							&bbpb.InvocationEvent{
								BuildEvent: &bespb.BuildEvent{
									Id: &bespb.BuildEventId{
										Id: &bespb.BuildEventId_Started{
											Started: &bespb.BuildEventId_BuildStartedId{},
										},
									},
								},
							},
						},
					},
				},
			},
			wantEvents: []*bespb.BuildEvent{
				&bespb.BuildEvent{
					Id: &bespb.BuildEventId{
						Id: &bespb.BuildEventId_Started{
							Started: &bespb.BuildEventId_BuildStartedId{},
						},
					},
				},
			},
		},
		{
			desc:         "zero invocations returned",
			invocationID: "180c8fc1-bfe1-444e-a00c-2d53768125b0",
			response: &bbpb.GetInvocationResponse{
				Invocation: []*bbpb.Invocation{},
			},
			wantErr: "returned 0 results",
		},
		{
			desc:         "two invocations returned",
			invocationID: "180c8fc1-bfe1-444e-a00c-2d53768125b0",
			response: &bbpb.GetInvocationResponse{
				Invocation: []*bbpb.Invocation{
					&bbpb.Invocation{
						Event: []*bbpb.InvocationEvent{
							&bbpb.InvocationEvent{
								BuildEvent: &bespb.BuildEvent{
									Id: &bespb.BuildEventId{
										Id: &bespb.BuildEventId_Started{
											Started: &bespb.BuildEventId_BuildStartedId{},
										},
									},
								},
							},
						},
					},
					&bbpb.Invocation{
						Event: []*bbpb.InvocationEvent{
							&bbpb.InvocationEvent{
								BuildEvent: &bespb.BuildEvent{
									Id: &bespb.BuildEventId{
										Id: &bespb.BuildEventId_Started{
											Started: &bespb.BuildEventId_BuildStartedId{},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: "returned 2 results",
		},
		{
			desc:         "HTTP error",
			invocationID: "180c8fc1-bfe1-444e-a00c-2d53768125b0",
			response: &bbpb.GetInvocationResponse{
				Invocation: []*bbpb.Invocation{
					&bbpb.Invocation{
						Event: []*bbpb.InvocationEvent{
							&bbpb.InvocationEvent{
								BuildEvent: &bespb.BuildEvent{
									Id: &bespb.BuildEventId{
										Id: &bespb.BuildEventId_Started{
											Started: &bespb.BuildEventId_BuildStartedId{},
										},
									},
								},
							},
						},
					},
				},
			},
			resCode: 500,
			wantErr: "HTTP response 500",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			testClient := newTestHttpClient(t, tc.resCode, tc.response)
			buildBuddy := &BuildBuddyClient{
				baseEndpoint: &url.URL{},
				httpClient:   testClient,
				apiKey:       "foobar",
			}

			got, gotErr := buildBuddy.GetBuildEvents(ctx, tc.invocationID)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, got, tc.wantEvents)
		})
	}
}

func TestTrivial(t *testing.T) {
	if 1 != 1 {
		t.Error("WTF")
	}
}