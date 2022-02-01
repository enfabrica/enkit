package kbuildbarn

import (
	"context"
	"github.com/enfabrica/enkit/lib/bes"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

type FilterOption func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList

func WithTestResults() FilterOption {
	return func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList {
		testResult := event.GetTestResult()
		if testResult != nil {
			return GenerateLinksForFiles(testResult.TestActionOutput, baseName, invocation, clusterName)
		}
		return nil
	}
}

func WithNamedSetOfFiles() FilterOption {
	return func(event *bespb.BuildEvent, baseName, invocation, clusterName string) HardlinkList {
		nsof := event.GetNamedSetOfFiles()
		if nsof != nil {
			return GenerateLinksForFiles(nsof.Files, baseName, invocation, clusterName)
		}
		return nil
	}
}

func GenerateSymlinks(ctx context.Context, client *bes.BuildBuddyClient, baseName, invocation, clusterName string, options ...FilterOption) (HardlinkList, error) {
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
