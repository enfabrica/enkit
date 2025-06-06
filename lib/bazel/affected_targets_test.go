package bazel

import (
	"fmt"
	"testing"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func testWorkspace(t *testing.T) *Workspace {
	t.Helper()
	files := map[string][]byte{
		"test/target/foo.txt":          []byte("Hello, world"),
		"test/target/foo_modified.txt": []byte("Goodbye, world"),
		"test/anotherdir/file.txt":     []byte("Another dir"),
	}
	sourceFS := testutil.NewFS(t, files)
	return &Workspace{
		sourceFS: sourceFS,
	}
}

func mustNewTarget(t *testing.T, target *bpb.Target) *Target {
	t.Helper()
	newTarget, err := NewTarget(testWorkspace(t), target, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create target: %v", err))
	}
	return newTarget
}

func mustNewPseudoTarget(t *testing.T, target *bpb.Target, events *WorkspaceEvents) *Target {
	t.Helper()
	newT, err := NewExternalPseudoTarget(testWorkspace(t), target, events)
	if err != nil {
		panic(fmt.Sprintf("failed to create target: %v", err))
	}
	return newT
}

func TestCalculateAffected(t *testing.T) {
	testCases := []struct {
		desc         string
		startResults *QueryResult
		endResults   *QueryResult
		want         []string
		wantErr      string
	}{
		{
			desc: "changed file detection",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test/target:foo.txt"),
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test/target:foo_modified.txt"), // Changed dependency, causes multiple hash changes
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo.txt",
			},
		},
		{
			desc: "dependency change propagation",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							RuleInput: []string{
								"//test/target:foo.txt",
							},
						},
					}),
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test/target:foo.txt"),
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							RuleInput: []string{
								"//test/target:foo.txt",
							},
						},
					}),
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test/target:foo_modified.txt"), // Changed contents, causes hash change
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo",
				"//test/target:foo.txt",
			},
		},
		{
			desc: "dependency cycle",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							RuleInput: []string{
								"//test/target:bar",
							},
						},
					}),
					"//test/target:bar": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:bar"),
							RuleInput: []string{
								"//test/target:foo.txt",
							},
						},
					}),
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test/target:foo.txt"),
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							RuleInput: []string{
								"//test/target:bar",
							},
						},
					}),
					"//test/target:bar": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:bar"),
							RuleInput: []string{
								"//test/target:foo",
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			wantErr: "dependency cycle",
		},
		{
			desc: "attribute change detection",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(1),
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(2), // Change, causes target change
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo",
			},
		},
		{
			desc: "attribute change detection",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(1),
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(2), // Change, causes target change
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo",
			},
		},
		{
			desc: "attribute reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(1),
								},
								{
									Name:     proto.String("int_attr_2"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(2),
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{ // Attributes reordered; no hash change
									Name:     proto.String("int_attr_2"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(2),
								},
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(1),
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "string list reorder detection",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_attr"),
									Type: bpb.Attribute_STRING_LIST.Enum(),
									StringListValue: []string{
										"value_1",
										"value_2",
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_attr"),
									Type: bpb.Attribute_STRING_LIST.Enum(),
									StringListValue: []string{ // Reordered; causes hash change
										"value_2",
										"value_1",
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo",
			},
		},
		{
			desc: "string dict reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_attr"),
									Type: bpb.Attribute_STRING_DICT.Enum(),
									StringDictValue: []*bpb.StringDictEntry{
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
										{
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_attr"),
									Type: bpb.Attribute_STRING_DICT.Enum(),
									StringDictValue: []*bpb.StringDictEntry{
										{ // Entries reordered; no hash change
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "label dict reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_dict_attr"),
									Type: bpb.Attribute_LABEL_DICT_UNARY.Enum(),
									LabelDictUnaryValue: []*bpb.LabelDictUnaryEntry{
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
										{
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_dict_attr"),
									Type: bpb.Attribute_LABEL_DICT_UNARY.Enum(),
									LabelDictUnaryValue: []*bpb.LabelDictUnaryEntry{
										{ // Entries reordered; no hash change
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "label list dict reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_list_dict_attr"),
									Type: bpb.Attribute_LABEL_LIST_DICT.Enum(),
									LabelListDictValue: []*bpb.LabelListDictEntry{
										{
											Key:   proto.String("foo"),
											Value: []string{"1"},
										},
										{
											Key:   proto.String("bar"),
											Value: []string{"2"},
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_list_dict_attr"),
									Type: bpb.Attribute_LABEL_LIST_DICT.Enum(),
									LabelListDictValue: []*bpb.LabelListDictEntry{
										{ // Entries reordered; no hash change
											Key:   proto.String("bar"),
											Value: []string{"2"},
										},
										{
											Key:   proto.String("foo"),
											Value: []string{"1"},
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "label keyed string dict reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_keyed_string_dict_attr"),
									Type: bpb.Attribute_LABEL_KEYED_STRING_DICT.Enum(),
									LabelKeyedStringDictValue: []*bpb.LabelKeyedStringDictEntry{
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
										{
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("label_keyed_string_dict_attr"),
									Type: bpb.Attribute_LABEL_KEYED_STRING_DICT.Enum(),
									LabelKeyedStringDictValue: []*bpb.LabelKeyedStringDictEntry{
										{ // Entries reordered; no hash change
											Key:   proto.String("bar"),
											Value: proto.String("2"),
										},
										{
											Key:   proto.String("foo"),
											Value: proto.String("1"),
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "string list dict reorder ignore",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_dict_attr"),
									Type: bpb.Attribute_STRING_LIST_DICT.Enum(),
									StringListDictValue: []*bpb.StringListDictEntry{
										{
											Key:   proto.String("foo"),
											Value: []string{"1"},
										},
										{
											Key:   proto.String("bar"),
											Value: []string{"2"},
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo"),
							Attribute: []*bpb.Attribute{
								{
									Name: proto.String("string_list_dict_attr"),
									Type: bpb.Attribute_STRING_LIST_DICT.Enum(),
									StringListDictValue: []*bpb.StringListDictEntry{
										{ // Entries reordered; no hash change
											Key:   proto.String("bar"),
											Value: []string{"2"},
										},
										{
											Key:   proto.String("foo"),
											Value: []string{"1"},
										},
									},
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
		{
			desc: "changed download hash invalidates external targets",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"@third_party_dep//:some_file.txt": mustNewPseudoTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("@third_party_dep//:some_file.txt"),
						},
					}, testWorkspace(t).ConstructWorkspaceEvents(map[string][]*bpb.WorkspaceEvent{
						"third_party_dep": {
							{
								Context: "repository @@third_party_dep",
								Event: &bpb.WorkspaceEvent_DownloadEvent{
									DownloadEvent: &bpb.DownloadEvent{
										Url:    []string{"https://example.com/some/url"},
										Sha256: "7a674b6a2b47f2c6dcf5e5375398fe1d959b60107bf561f7c754f5c09d1163db",
									},
								},
							},
						},
					})),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"@third_party_dep//:some_file.txt": mustNewPseudoTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("@third_party_dep//:some_file.txt"),
						},
					}, testWorkspace(t).ConstructWorkspaceEvents(map[string][]*bpb.WorkspaceEvent{
						"third_party_dep": {
							{
								Context: "repository @@third_party_dep",
								Event: &bpb.WorkspaceEvent_DownloadEvent{
									DownloadEvent: &bpb.DownloadEvent{
										Url:    []string{"https://example.com/some/url"},
										Sha256: "5279ebd204a4e36501c4b6d061890a7fff76d6c43610f121c91ef61b38d0e011",
									},
								},
							},
						},
					})),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"@third_party_dep//:some_file.txt",
			},
		},
		{
			desc: "generated file depends on generating rule",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_GENERATED_FILE.Enum(),
						GeneratedFile: &bpb.GeneratedFile{
							Name:           proto.String("//test/target:foo.txt"),
							GeneratingRule: proto.String("//test/target:foo_rule"),
						},
					}),
					"//test/target:foo_rule": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo_rule"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(1),
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test/target:foo.txt": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_GENERATED_FILE.Enum(),
						GeneratedFile: &bpb.GeneratedFile{
							Name:           proto.String("//test/target:foo.txt"),
							GeneratingRule: proto.String("//test/target:foo_rule"),
						},
					}),
					"//test/target:foo_rule": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_RULE.Enum(),
						Rule: &bpb.Rule{
							Name: proto.String("//test/target:foo_rule"),
							Attribute: []*bpb.Attribute{
								{
									Name:     proto.String("int_attr"),
									Type:     bpb.Attribute_INTEGER.Enum(),
									IntValue: proto.Int32(2), // Changed attr value; should affect this rule and generated file
								},
							},
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{
				"//test/target:foo.txt",
				"//test/target:foo_rule",
			},
		},
		{
			desc: "source file that points to dir",
			startResults: &QueryResult{
				Targets: map[string]*Target{
					"//test:target": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test:target"), // Actually a directory; shouldn't cause an error
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			endResults: &QueryResult{
				Targets: map[string]*Target{
					"//test:target": mustNewTarget(t, &bpb.Target{
						Type: bpb.Target_SOURCE_FILE.Enum(),
						SourceFile: &bpb.SourceFile{
							Name: proto.String("//test:target"),
						},
					}),
				},
				workspace: testWorkspace(t),
			},
			want: []string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got, gotErr := calculateAffected(tc.startResults, tc.endResults)

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}

			assert.Equal(t, tc.want, got)
		})
	}
}
