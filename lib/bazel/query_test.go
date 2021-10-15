package bazel

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"testing"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/errdiff"

	rulesgo "github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func mustFindRunfile(path string) string {
	p, err := rulesgo.Runfile(path)
	if err != nil {
		panic(fmt.Sprintf("can't find runfile %q: %v", path, err))
	}
	return p
}

func TestQueryOutput(t *testing.T) {
	testCases := []struct {
		desc            string
		queryOutputFile string
		wantCount       int
		wantErr         string
	}{
		{
			desc:            "query deps //lib/bazel/commands/...",
			queryOutputFile: mustFindRunfile("lib/bazel/testdata/query_deps_lib_bazel_commands.pb"),
			wantCount:       1740,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&streamedBazelCommand, func(*exec.Cmd) (io.Reader, chan error, error) {
				errChan := make(chan error)
				close(errChan)
				contents, err := ioutil.ReadFile(tc.queryOutputFile)
				if err != nil {
					panic(fmt.Sprintf("failed to read query output test file %q: %v", tc.queryOutputFile, err))
				}
				return bytes.NewReader(contents), errChan, nil
			})
			defer stubs.Reset()

			w, err := OpenWorkspace("")
			if err != nil {
				t.Errorf("got error while opening workspace: %v; want no error", err)
				return
			}

			got, gotErr := w.Query("") // args don't matter

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}

			assert.Equal(t, tc.wantCount, len(got.Targets))
		})
	}
}

func TestQueryResultAddChecksumAttributeToExternals(t *testing.T) {
	testCases := []struct {
		desc            string
		targets         map[string]*Target
		workspaceEvents map[string][]*bpb.WorkspaceEvent
		wantTargets     map[string]*Target
		wantErr         string
	}{
		{
			desc: "doesn't affect rules with no download events",
			targets: map[string]*Target{
				"//some:target": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							RuleInput: []string{
								"//some:dependency",
								"@third_party//:dependency",
							},
							RuleOutput: []string{
								"//some:output.txt",
							},
						},
					},
				},
				"//some/other:target": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							RuleInput: []string{
								"//some/other:dependency",
								"@third_party//yet/another:dependency",
							},
							RuleOutput: []string{
								"//some/other:output.txt",
							},
						},
					},
				},
			},
			workspaceEvents: map[string][]*bpb.WorkspaceEvent{},
			wantTargets: map[string]*Target{
				"//some:target": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							RuleInput: []string{
								"//some:dependency",
								"@third_party//:dependency",
							},
							RuleOutput: []string{
								"//some:output.txt",
							},
						},
					},
				},
				"//some/other:target": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							RuleInput: []string{
								"//some/other:dependency",
								"@third_party//yet/another:dependency",
							},
							RuleOutput: []string{
								"//some/other:output.txt",
							},
						},
					},
				},
			},
		},
		{
			desc: "adds attribute to external dependency",
			targets: map[string]*Target{
				"@third_party//some:dependency_2": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("@third_party//some:dependency_2"),
							RuleInput: []string{
								"//some:dependency",
								"@third_party//:dependency",
							},
						},
					},
				},
			},
			workspaceEvents: map[string][]*bpb.WorkspaceEvent{
				"//external:some_dependency": []*bpb.WorkspaceEvent{
					&bpb.WorkspaceEvent{
						Rule: "//external:some_dependency",
						Event: &bpb.WorkspaceEvent_DownloadEvent{
							DownloadEvent: &bpb.DownloadEvent{
								Url:    []string{"https://example.com/some/url"},
								Sha256: "7a674b6a2b47f2c6dcf5e5375398fe1d959b60107bf561f7c754f5c09d1163db",
							},
						},
					},
					&bpb.WorkspaceEvent{
						Rule: "//external:some_dependency",
						Event: &bpb.WorkspaceEvent_DownloadAndExtractEvent{
							DownloadAndExtractEvent: &bpb.DownloadAndExtractEvent{
								Url:    []string{"https://example.com/some/other/url"},
								Sha256: "5279ebd204a4e36501c4b6d061890a7fff76d6c43610f121c91ef61b38d0e011",
							},
						},
					},
				},
			},
			wantTargets: map[string]*Target{
				"@third_party//some:dependency_2": &Target{
					Target: &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("@third_party//some:dependency_2"),
							RuleInput: []string{
								"//some:dependency",
								"@third_party//:dependency",
							},
							Attribute: []*bpb.Attribute{
								&bpb.Attribute{
									Name: proto.String("workspace_download_checksums"),
									StringListValue: []string{
										"7a674b6a2b47f2c6dcf5e5375398fe1d959b60107bf561f7c754f5c09d1163db",
										"5279ebd204a4e36501c4b6d061890a7fff76d6c43610f121c91ef61b38d0e011",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			qr := &QueryResult{
				Targets:         tc.targets,
				WorkspaceEvents: tc.workspaceEvents,
			}

			gotErr := qr.addChecksumsAttributeToExternals()

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assert.Equal(t, tc.wantTargets, qr.Targets)
		})
	}
}
