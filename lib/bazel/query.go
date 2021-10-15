package bazel

import (
	"fmt"
	"io"
	"log"
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
	log.Println("Adding external target checksum attribute...")
	err := r.addChecksumsAttributeToExternals()
	if err != nil {
		return nil, fmt.Errorf("failed to add checksums to external targets: %w", err)
	}

	log.Println("Evaluating dependencies...")
	err = fillDependencies(r.Targets)
	if err != nil {
		return nil, fmt.Errorf("failed to link dependencies: %w", err)
	}

	log.Println("Hashing targets...")
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
	for name, t := range targets {
		if t.Target.GetType() != bpb.Target_RULE {
			continue
		}
		deps := t.Target.GetRule().GetRuleInput()
		for _, dep := range deps {
			depTarget, ok := targets[dep]
			if !ok {
				log.Printf("target %q: dependency %q not found in targets; skipping", name, dep)
				continue
			}
			t.deps = append(t.deps, depTarget)
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
		*target.Target = bpb.Target{
			Type: bpb.Target_RULE.Enum(),
			Rule: &bpb.Rule{
				Name:      &targetName,
				RuleInput: deps,
				Attribute: []*bpb.Attribute{
					&bpb.Attribute{
						Name:            proto.String("workspace_download_checksums"),
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

	cmd := w.bazelCommand(queryOpts)
	resultStream, errChan, err := streamedBazelCommand(cmd)
	if err != nil {
		return nil, err
	}

	targets := map[string]*Target{}

	rdr := delimited.NewReader(resultStream)
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
		return nil, fmt.Errorf("failed to read stdout from bazel command: %w", err)
	}

	if err := queryOpts.filterError(<-errChan); err != nil {
		return nil, err
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

func coarseExternalTargets(all []*Label) []*Label {
	dedup := map[string]*Label{}
	for _, l := range all {
		if l.isExternal() {
			ext := l.toCoarseExternal()
			dedup[ext.String()] = ext
		}
	}

	var ret []*Label
	for _, v := range dedup {
		ret = append(ret, v)
	}
	return ret
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
