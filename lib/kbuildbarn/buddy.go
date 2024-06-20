package kbuildbarn

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/enfabrica/enkit/lib/bes"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

type FilterOption func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList

func outputDirForTest(label string, run int32, attempt int32) string {
	components := strings.Split(label, "//")
	if len(components) != 2 {
		return ""
	}
	return filepath.Join(
		"bazel-testlogs",
		strings.Replace(components[1], ":", "/", -1),
		fmt.Sprintf("run_%d", run),
		fmt.Sprintf("attempt_%d", attempt),
	)
}

func WithTestResults() FilterOption {
	return func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList {
		testResult := event.GetTestResult()
		if testResult != nil {
			// Files that are typically in a test.outputs subdirectory come to this
			// name field with the basename joined to the dirname with `__`. Turn this
			// back into a path so we don't end up with paths like
			// `test.outputs__outputs.zip`
			for _, tao := range testResult.TestActionOutput {
				tao.Name = strings.Replace(tao.Name, "__", "/", -1)
			}

			return GenerateLinksForFiles(
				testResult.TestActionOutput,
				baseName,
				outputDirForTest(
					event.GetId().GetTestResult().GetLabel(),
					event.GetId().GetTestResult().GetRun(),
					event.GetId().GetTestResult().GetShard(),
				),
				invocation,
				clusterName,
			)
		}
		return nil
	}
}

func WithNamedSetOfFiles() FilterOption {
	return func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList {
		nsof := event.GetNamedSetOfFiles()
		if nsof != nil {
			return GenerateLinksForFiles(nsof.Files, baseName, "", invocation, clusterName)
		}
		return nil
	}
}

func GenerateHardlinks(ctx context.Context, client *bes.BuildBuddyClient, baseName, clusterName, invocation string, options ...FilterOption) (HardlinkList, error) {
	result, err := client.GetBuildEvents(ctx, invocation)
	if err != nil {
		return nil, err
	}
	var parsedResults []HardlinkList
	for _, event := range result {
		for _, fOpt := range options {
			parsedResults = append(parsedResults, fOpt(event, baseName, invocation, clusterName))
		}
	}
	return MergeLists(parsedResults...), nil
}
