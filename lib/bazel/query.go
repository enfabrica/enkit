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
	Events 			*WorkspaceEvents

	workspace *Workspace
}

func (r *QueryResult) TargetHashes() (TargetHashes, error) {
	err := fillDependencies(r.Targets)
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
		if err := t.ResolveDeps(targets); err != nil {
			return fmt.Errorf("failed to fill deps for target %q: %w", t.Name(), err)
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

	var workspaceEvents *WorkspaceEvents
	if queryOpts.workspaceLog != nil {
		workspaceEvents, err = ParseWorkspaceEvents(queryOpts.workspaceLog)
		if err != nil {
			return nil, err
		}
	}

	stdout, err := cmd.Stdout()
	if err != nil {
		return nil, fmt.Errorf("failed to open query stdout: %w", err)
	}
	defer stdout.Close()
	rdr := delimited.NewReader(stdout)

	targets := map[string]*Target{}
	var buf []byte
	for buf, err = rdr.Next(); err == nil; buf, err = rdr.Next() {
		var target bpb.Target
		if err := proto.Unmarshal(buf, &target); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Target message: %w", err)
		}
		newTarget, err := NewTarget(w, &target, workspaceEvents)
		if err != nil {
			return nil, err
		}
		targets[newTarget.Name()] = newTarget
	}
	if err != io.EOF {
		return nil, fmt.Errorf("error while reading stdout from bazel command: %w", err)
	}

	return &QueryResult{
		Targets:         targets,
		Events: workspaceEvents,
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

func (l *Label) filePath() string {
	if len(l.Workspace) == 0 {
		return filepath.Join(l.Package, l.Rule)
	}
	return filepath.Join("external", l.Workspace, l.Package, l.Rule)
}

func (l *Label) isExternal() bool {
	return l.Workspace != "" || l.Package == "external"
}

func (l *Label) WorkspaceName() string {
	if len(l.Workspace) != 0 {
		return l.Workspace
	}
	return ""
}
