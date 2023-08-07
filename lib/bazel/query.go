package bazel

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/proto/delimited"

	"google.golang.org/protobuf/proto"
)

type QueryResult struct {
	Targets         map[string]*Target
	WorkspaceEvents map[string][]*bpb.WorkspaceEvent

	workspace *Workspace
}

func (r *QueryResult) TargetHashes() (TargetHashes, error) {
	// Add a single attribute to each external target with the sorted SHA256 sums
	// of all their downloads. This will allow for the subsequent hashing to
	// change the hash for the entire coarse external target if any of the
	// download SHA256 sums change.
	err := r.addChecksumsAttributeToExternals()
	if err != nil {
		return nil, fmt.Errorf("failed to add checksums to external targets: %w", err)
	}

	err = fillDependencies(r.Targets)
	if err != nil {
		return nil, fmt.Errorf("failed to link dependencies: %w", err)
	}

	hashes := TargetHashes(map[string]uint32{})
	for name, t := range r.Targets {
		h, err := t.getHash(r.workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to get hash for %q: %w", name, err)
		}
		hashes[name] = h
	}
	return hashes, nil
}

func fillDependencies(targets map[string]*Target) error {
	for _, t := range targets {
		switch t.Target.GetType() {
		case bpb.Target_RULE:
			deps := t.Target.GetRule().GetRuleInput()
			for _, dep := range deps {
				depTarget, ok := targets[dep]
				if !ok {
					// TODO(scott): Log this condition
					return fmt.Errorf("dep %q not found for target %q", dep, t.Name())
					continue
				}
				t.deps = append(t.deps, depTarget)
			}
		case bpb.Target_GENERATED_FILE:
			dep := t.Target.GetGeneratedFile().GetGeneratingRule()
			depTarget, ok := targets[dep]
			if !ok {
				// TODO(scott): Log this condition
				return fmt.Errorf("dep %q not found for target %q", dep, t.Name())
				continue
			}
			t.deps = append(t.deps, depTarget)
		default:
			continue
		}
	}
	return nil
}

func (r *QueryResult) addChecksumsAttributeToExternals() error {
	for targetName, target := range r.Targets {
		lbl, err := labelFromString(targetName)
		if err != nil {
			return err
		}
		if !lbl.isExternal() {
			continue
		}

		lbl = lbl.toCoarseExternal()
		events := r.WorkspaceEvents[lbl.String()]

		var checksums []string
		for _, event := range events {
			switch e := event.GetEvent().(type) {
			case *bpb.WorkspaceEvent_DownloadEvent:
				if e.DownloadEvent.GetSha256() != "" {
					checksums = append(checksums, e.DownloadEvent.GetSha256())
				}
				if e.DownloadEvent.GetIntegrity() != "" {
					checksums = append(checksums, e.DownloadEvent.GetIntegrity())
				}
			case *bpb.WorkspaceEvent_DownloadAndExtractEvent:
				if e.DownloadAndExtractEvent.GetSha256() != "" {
					checksums = append(checksums, e.DownloadAndExtractEvent.GetSha256())
				}
				if e.DownloadAndExtractEvent.GetIntegrity() != "" {
					checksums = append(checksums, e.DownloadAndExtractEvent.GetIntegrity())
				}
			case *bpb.WorkspaceEvent_ExecuteEvent:
				if len(e.ExecuteEvent.GetArguments()) == 2 && e.ExecuteEvent.GetArguments()[0] == "echo" {
					fmt.Printf("got checksum: %q\n", e.ExecuteEvent.GetArguments()[1])
					checksums = append(checksums, e.ExecuteEvent.GetArguments()[1])
				}
			}
		}

		var deps []string
		if target.GetType() == bpb.Target_RULE {
			deps = target.GetRule().GetRuleInput()
		}

		// Rewrite the target as a "rule" that only has an attribute that
		// corresponds to the checksum(s) used when downloading the repo in which it
		// resides. Therefore, this target's hash will change iff one or more
		// downloads for its repo has a checksum change.
		nameCopy := targetName
		*target.Target = bpb.Target{
			Type: bpb.Target_RULE.Enum(),
			Rule: &bpb.Rule{
				Name:      &nameCopy,
				RuleInput: deps,
				Attribute: []*bpb.Attribute{
					&bpb.Attribute{
						Name:            proto.String("workspace_download_checksums"),
						Type:            bpb.Attribute_STRING_LIST.Enum(),
						StringListValue: checksums,
					},
				},
			},
		}
	}
	return nil
}

// Query performs a `bazel query` using the provided query string. If
// `keep_going` is set, then `--keep_going` is set on the bazel commandline, and
// errors from the bazel process are ignored.
func (w *Workspace) Query(query string, options ...QueryOption) (*QueryResult, error) {
	queryOpts := &queryOptions{query: query}
	QueryOptions(options).apply(queryOpts)
	defer queryOpts.Close()

	cmd, err := w.bazelCommand(queryOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}
	defer cmd.Close()
	err = cmd.Run()
	if err := queryOpts.filterError(err); err != nil {
		return nil, fmt.Errorf("Command: %s\nError: %v\n\nbazel stderr:\n%s", cmd.String(), err, cmd.StderrContents())
	}

	targets := map[string]*Target{}

	stdout, err := cmd.Stdout()
	if err != nil {
		return nil, fmt.Errorf("failed to open query stdout: %w", err)
	}
	defer stdout.Close()
	rdr := delimited.NewReader(stdout)

	var buf []byte
	for buf, err = rdr.Next(); err == nil; buf, err = rdr.Next() {
		var target bpb.Target
		if err := proto.Unmarshal(buf, &target); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Target message: %w", err)
		}
		newTarget := &Target{Target: &target}
		targets[newTarget.Name()] = newTarget
	}
	if err != io.EOF {
		return nil, fmt.Errorf("error while reading stdout from bazel command: %w", err)
	}

	workspaceEvents := map[string][]*bpb.WorkspaceEvent{}
	if queryOpts.workspaceLog != nil {
		rdr := delimited.NewReader(queryOpts.workspaceLog)
		var buf []byte
		for buf, err = rdr.Next(); err == nil; buf, err = rdr.Next() {
			var event bpb.WorkspaceEvent
			if err := proto.Unmarshal(buf, &event); err != nil {
				return nil, fmt.Errorf("failed to unmarshal WorkspaceEvent message: %w", err)
			}
			workspaceEvents[event.GetRule()] = append(workspaceEvents[event.GetRule()], &event)
		}
	}

	return &QueryResult{
		Targets:         targets,
		WorkspaceEvents: workspaceEvents,
		workspace:       w,
	}, nil
}

func labels(t map[string]*bpb.Target) ([]*Label, error) {
	var ext []*Label
	for k := range t {
		l, err := labelFromString(k)
		if err != nil {
			return nil, err
		}
		ext = append(ext, l)
	}
	return ext, nil
}

type Label struct {
	Workspace string
	Package   string
	Rule      string
}

func labelFromString(labelStr string) (*Label, error) {
	l := &Label{}
	pieces := strings.Split(labelStr, "//")
	if len(pieces) != 2 {
		return nil, fmt.Errorf("label %q is malformed; want one instance of '//'", labelStr)
	}
	l.Workspace = ""
	if strings.HasPrefix(pieces[0], "@") {
		l.Workspace = pieces[0][1:]
	}
	pieces = strings.Split(pieces[1], ":")
	if len(pieces) != 2 {
		return nil, fmt.Errorf("label %q is malformed; want one instance of ':'", labelStr)
	}
	l.Package = pieces[0]
	l.Rule = pieces[1]
	return l, nil
}

func (l *Label) String() string {
	var b strings.Builder
	if l.Workspace != "" {
		fmt.Fprintf(&b, "@%s", l.Workspace)
	}
	fmt.Fprintf(&b, "//%s:%s", l.Package, l.Rule)
	return b.String()
}

func (l *Label) toCoarseExternal() *Label {
	return &Label{
		Package: "external",
		Rule:    l.Workspace,
	}
}

func (l *Label) filePath() string {
	if l.Workspace != "" {
		panic(fmt.Sprintf("shouldn't be looking up generated files in //external: %+v", l))
	}
	return filepath.Join(l.Package, l.Rule)
}

func (l *Label) isExternal() bool {
	return l.Workspace != "" || l.Package == "external"
}
