package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/enfabrica/enkit/lib/errdiff"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

type buddyEvent struct {
	SequenceNumber string
	BuildEvent     json.RawMessage
}

func unmarshalEventsList(f fs.File) ([]*bespb.BuildEvent, error) {
	var events []*bespb.BuildEvent

	var arr []buddyEvent
	if err := json.NewDecoder(f).Decode(&arr); err != nil {
		return nil, fmt.Errorf("failed to decode array: %w", err)
	}

	for _, msg := range arr {
		bytes, err := json.Marshal(msg.BuildEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to encode RawMessage #%s back to bytes: %w", msg.SequenceNumber, err)
		}
		event := &bespb.BuildEvent{}
		if err := protojson.Unmarshal(bytes, event); err != nil {
			fmt.Println(string(bytes))
			return nil, fmt.Errorf("failed to unmarshal BuildEvent #%s from JSON: %w", msg.SequenceNumber, err)
		}
		events = append(events, event)
	}

	return events, nil
}

func TestTargetStatusFromBuildEvents(t *testing.T) {
	testCases := []struct {
		desc           string
		eventsFilepath string
		want           *invocation
		wantErr        string
	}{
		{
			desc:           "successful build",
			eventsFilepath: "enkit/lib/bazel/commands/testdata/success.json",
			want: &invocation{
				finished: true,
				targets: map[string]*target{
					"//lib/config/datastore:datastore": {
						name:   "//lib/config/datastore:datastore",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/datastore:go_default_library": {
						name:   "//lib/config/datastore:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/defcon:defcon": {
						name:   "//lib/config/defcon:defcon",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/defcon:go_default_library": {
						name:   "//lib/config/defcon:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/directory:directory": {
						name:   "//lib/config/directory:directory",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/directory:directory_test": {
						name:   "//lib/config/directory:directory_test",
						status: bespb.TestStatus_PASSED,
						rule:   "go_test",
						isTest: true,
					},
					"//lib/config/directory:go_default_library": {
						name:   "//lib/config/directory:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/identity:go_default_library": {
						name:   "//lib/config/identity:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/identity:identity": {
						name:   "//lib/config/identity:identity",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/marshal:go_default_library": {
						name:   "//lib/config/marshal:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/marshal:marshal": {
						name:   "//lib/config/marshal:marshal",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/marshal:marshal_test": {
						name:   "//lib/config/marshal:marshal_test",
						status: bespb.TestStatus_PASSED,
						rule:   "go_test",
						isTest: true,
					},
					"//lib/config/remote:go_default_library": {
						name:   "//lib/config/remote:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//lib/config/remote:remote": {
						name:   "//lib/config/remote:remote",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config/remote:remote_test": {
						name:   "//lib/config/remote:remote_test",
						status: bespb.TestStatus_PASSED,
						rule:   "go_test",
						isTest: true,
					},
					"//lib/config:config": {
						name:   "//lib/config:config",
						status: bespb.TestStatus_PASSED,
						rule:   "go_library",
						isTest: false,
					},
					"//lib/config:config_test": {
						name:   "//lib/config:config_test",
						status: bespb.TestStatus_PASSED,
						rule:   "go_test",
						isTest: true,
					},
					"//lib/config:go_default_library": {
						name:   "//lib/config:go_default_library",
						status: bespb.TestStatus_PASSED,
						rule:   "alias",
						isTest: false,
					},
					"//aborted/target:one": {
						name:   "//aborted/target:one",
						rule:   "go_library",
						status: bespb.TestStatus_INCOMPLETE,
						isTest: false,
					},
					"//aborted/target:two": {
						name:   "//aborted/target:two",
						rule:   "go_library",
						status: bespb.TestStatus_INCOMPLETE,
						isTest: false,
					},
					"//aborted/target:three": {
						name:   "//aborted/target:three",
						rule:   "go_library",
						status: bespb.TestStatus_INCOMPLETE,
						isTest: false,
					},
				},
			},
		},
	}
	r, err := runfiles.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := r.Open(tc.eventsFilepath)
			require.NoError(t, err)

			events, err := unmarshalEventsList(f)
			require.NoError(t, err)

			got, gotErr := invocationStatusFromBuildEvents(events)
			errdiff.Check(t, gotErr, tc.wantErr)

			if gotErr != nil {
				return
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTargetStatusFilter(t *testing.T) {
	targetSet := map[string]*target{
		"//lib/config/datastore:datastore": {
			name:   "//lib/config/datastore:datastore",
			rule:   "go_library",
			status: bespb.TestStatus_PASSED,
			isTest: false,
		},
		"//lib/config/datastore:go_default_library": {
			name:   "//lib/config/datastore:go_default_library",
			rule:   "go_library",
			status: bespb.TestStatus_INCOMPLETE,
			isTest: false,
		},
		"//lib/config/defcon:defcon": {
			name:   "//lib/config/defcon:defcon",
			rule:   "go_library",
			status: bespb.TestStatus_NO_STATUS,
			isTest: false,
		},
		"//lib/config/defcon:go_default_library": {
			name:   "//lib/config/defcon:go_default_library",
			rule:   "go_library",
			status: bespb.TestStatus_FLAKY,
			isTest: false,
		},
		"//lib/config/directory:directory": {
			name:   "//lib/config/directory:directory",
			rule:   "go_library",
			status: bespb.TestStatus_TIMEOUT,
			isTest: false,
		},
		"//lib/config/directory:directory_test": {
			name:   "//lib/config/directory:directory_test",
			rule:   "go_test",
			status: bespb.TestStatus_REMOTE_FAILURE,
			isTest: true,
		},
		"//lib/config/directory:go_default_library": {
			name:   "//lib/config/directory:go_default_library",
			rule:   "go_library",
			status: bespb.TestStatus_FAILED_TO_BUILD,
			isTest: false,
		},
		"//lib/config/identity:go_default_library": {
			name:   "//lib/config/identity:go_default_library",
			rule:   "go_library",
			status: bespb.TestStatus_TOOL_HALTED_BEFORE_TESTING,
			isTest: false,
		},
		"//lib/config/identity:identity": {
			name:   "//lib/config/identity:identity",
			rule:   "go_library",
			status: bespb.TestStatus_PASSED,
			isTest: false,
		},
		"//lib/config/marshal:go_default_library": {
			name:   "//lib/config/marshal:go_default_library",
			rule:   "go_library",
			status: bespb.TestStatus_PASSED,
			isTest: false,
		},
	}

	testCases := []struct {
		desc    string
		start   *invocation
		filter  []string
		want    *invocation
		wantErr string
	}{
		{
			desc: "single filter",
			start: &invocation{
				targets: targetSet,
			},
			filter: []string{"passed"},
			want: &invocation{
				targets: map[string]*target{
					"//lib/config/datastore:datastore": {
						name:   "//lib/config/datastore:datastore",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/identity:identity": {
						name:   "//lib/config/identity:identity",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/marshal:go_default_library": {
						name:   "//lib/config/marshal:go_default_library",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
				},
			},
		},
		{
			desc: "multi filter",
			start: &invocation{
				targets: targetSet,
			},
			filter: []string{"passed", "timeout"},
			want: &invocation{
				targets: map[string]*target{
					"//lib/config/datastore:datastore": {
						name:   "//lib/config/datastore:datastore",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/identity:identity": {
						name:   "//lib/config/identity:identity",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/marshal:go_default_library": {
						name:   "//lib/config/marshal:go_default_library",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/directory:directory": {
						name:   "//lib/config/directory:directory",
						rule:   "go_library",
						status: bespb.TestStatus_TIMEOUT,
						isTest: false,
					},
				},
			},
		},
		{
			desc: "no filter",
			start: &invocation{
				targets: targetSet,
			},
			filter: []string{},
			want: &invocation{
				targets: targetSet,
			},
		},
		{
			desc: "preserves finished",
			start: &invocation{
				finished: true,
				targets:  targetSet,
			},
			filter: []string{"passed"},
			want: &invocation{
				finished: true,
				targets: map[string]*target{
					"//lib/config/datastore:datastore": {
						name:   "//lib/config/datastore:datastore",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/identity:identity": {
						name:   "//lib/config/identity:identity",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
					"//lib/config/marshal:go_default_library": {
						name:   "//lib/config/marshal:go_default_library",
						rule:   "go_library",
						status: bespb.TestStatus_PASSED,
						isTest: false,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.start.Filter(tc.filter...)

			assert.Equal(t, tc.want, got)
		})
	}
}
