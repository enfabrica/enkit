package commands

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

// Assume that tests end in `_test`
// BUG: DV regression tests also add a number suffix, like `test_12`.
var labelIsTestRegexp = regexp.MustCompile(`_test(_\d+)?$`)

func validStatuses() []string {
	valid := maps.Keys(bespb.TestStatus_value)
	for i := range valid {
		valid[i] = strings.ToLower(valid[i])
	}
	return valid
}

func parseRuleType(r string) string {
	return strings.TrimSuffix(r, " rule")
}

type target struct {
	name   string
	status bespb.TestStatus
	rule   string
	isTest bool
}

type invocation struct {
	targets  map[string]*target
	finished bool
}

func emptyInvocationStatus() *invocation {
	return &invocation{
		targets:  map[string]*target{},
		finished: false,
	}
}

func invocationStatusFromBuildEvents(events []*bespb.BuildEvent) (*invocation, error) {
	new := emptyInvocationStatus()

	for _, event := range events {
		switch id := event.Id.Id.(type) {
		case *bespb.BuildEventId_TargetConfigured:
			new.targets[id.TargetConfigured.GetLabel()] = &target{
				name:   id.TargetConfigured.GetLabel(),
				status: bespb.TestStatus_NO_STATUS,
				rule:   parseRuleType(event.GetConfigured().GetTargetKind()),
				isTest: event.GetConfigured().GetTestSize() != bespb.TestSize_UNKNOWN,
			}

		// BUG(INFRA-9875): For events below - aborted events can come in with
		// various event ID types, so only switching on the event ID type is
		// insufficient.

		case *bespb.BuildEventId_TargetCompleted:
			switch event.Payload.(type) {
			case *bespb.BuildEvent_Completed:
				lbl := id.TargetCompleted.GetLabel()
				// This is the final message for build-only targets; test targets will
				// have an additional TestResult message with the status of the test.
				//
				// TODO(scott): This won't be true for `bazel build` commands - maybe we
				// should track build and test status separately?
				if !new.targets[lbl].isTest {
					if event.GetCompleted().GetSuccess() {
						new.targets[lbl].status = bespb.TestStatus_PASSED
					} else {
						new.targets[lbl].status = bespb.TestStatus_FAILED
					}
				}

			case *bespb.BuildEvent_Aborted:
				new.targets[id.TargetCompleted.GetLabel()].status = bespb.TestStatus_INCOMPLETE
			}

		case *bespb.BuildEventId_TestSummary:
			switch payload := event.Payload.(type) {
			case *bespb.BuildEvent_TestSummary:
				status := payload.TestSummary.GetOverallStatus()
				new.targets[id.TestSummary.GetLabel()].status = status

			case *bespb.BuildEvent_Aborted:
				new.targets[id.TestSummary.GetLabel()].status = bespb.TestStatus_INCOMPLETE
			}

		case *bespb.BuildEventId_TestResult:
			switch event.Payload.(type) {
			case *bespb.BuildEvent_Aborted:
				new.targets[id.TestResult.GetLabel()].status = bespb.TestStatus_INCOMPLETE
			}

		case *bespb.BuildEventId_BuildFinished:
			new.finished = true

		}
	}

	return new, nil
}

func (t *invocation) Filter(statuses ...string) *invocation {
	if len(statuses) == 0 {
		return t
	}

	statusMap := map[bespb.TestStatus]struct{}{}
	for _, s := range statuses {
		if val, ok := bespb.TestStatus_value[strings.ToUpper(s)]; ok {
			statusMap[bespb.TestStatus(val)] = struct{}{}
		}
	}

	new := &invocation{
		finished: t.finished,
		targets:  map[string]*target{},
	}
	for tname, tStatus := range t.targets {
		if _, ok := statusMap[tStatus.status]; ok {
			new.targets[tname] = tStatus
		}
	}

	return new
}

func (t *invocation) Sorted() []string {
	targets := maps.Keys(t.targets)
	sort.Strings(targets)
	return targets
}

func (t *invocation) PrettyPrint(w io.Writer) error {
	running := t.Filter("no_status")
	passed := t.Filter("passed")
	flaky := t.Filter("flaky")
	timedOut := t.Filter("timeout")
	failed := t.Filter("failed")
	incomplete := t.Filter("incomplete")
	infraFailure := t.Filter("remote_failure")
	buildFailure := t.Filter("failed_to_build")
	toolHalted := t.Filter("tool_halted_before_testing")

	// When printing, print the more important failures towards the bottom, which
	// are more visible to the user.

	if t.finished {
		// Successes are the least interesting when the invocation has finished
		if len(passed.targets) > 0 {
			prettyPrintTargetList(w, passed, "Completed targets", "🟢")
		}
		if len(incomplete.targets) > 0 {
			prettyPrintTargetList(w, incomplete, "Incomplete/aborted targets", "❓")
		}
		if len(flaky.targets) > 0 {
			prettyPrintTargetList(w, flaky, "Flaky targets", "🪫")
		}
		if len(toolHalted.targets) > 0 {
			prettyPrintTargetList(w, toolHalted, "Halted before testing", "🛑")
		}
		if len(failed.targets) > 0 {
			prettyPrintTargetList(w, failed, "Failed targets", "❌")
		}
		if len(timedOut.targets) > 0 {
			prettyPrintTargetList(w, timedOut, "Timed out targets", "⏰")
		}
		if len(infraFailure.targets) > 0 {
			prettyPrintTargetList(w, infraFailure, "Infra failures", "🚨")
		}
		if len(buildFailure.targets) > 0 {
			prettyPrintTargetList(w, buildFailure, "Broken targets", "💣")
		}
	} else {
		if len(passed.targets) > 0 {
			prettyPrintTargetList(w, passed, "Completed targets", "🟢")
		}
		if len(incomplete.targets) > 0 {
			prettyPrintTargetList(w, incomplete, "Incomplete targets", "❓")
		}
		if len(flaky.targets) > 0 {
			prettyPrintTargetList(w, flaky, "Flaky targets", "🪫")
		}
		if len(toolHalted.targets) > 0 {
			prettyPrintTargetList(w, toolHalted, "Halted before testing", "🛑")
		}
		if len(failed.targets) > 0 {
			prettyPrintTargetList(w, failed, "Failed targets", "❌")
		}
		if len(timedOut.targets) > 0 {
			prettyPrintTargetList(w, timedOut, "Timed out targets", "⏰")
		}
		if len(infraFailure.targets) > 0 {
			prettyPrintTargetList(w, infraFailure, "Infra failures", "🚨")
		}
		if len(buildFailure.targets) > 0 {
			prettyPrintTargetList(w, buildFailure, "Broken targets", "💣")
		}
		if len(running.targets) > 0 {
			prettyPrintTargetList(w, running, "Running targets", "🟡")
		}
	}

	return nil
}

func prettyPrintTargetList(w io.Writer, inv *invocation, header string, icon string) {
	fmt.Fprintf(w, "%s (%d):\n", header, len(inv.targets))
	for _, t := range inv.Sorted() {
		fmt.Fprintf(w, "%s %s\n", icon, t)
	}
	fmt.Fprintln(w, "")
}

func (t *invocation) Print(w io.Writer) error {
	for _, name := range t.Sorted() {
		fmt.Fprintln(w, name)
	}
	return nil
}
