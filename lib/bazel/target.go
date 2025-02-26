package bazel

import (
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
)

var (
	pseudoTargetAttributeName = "workspace_download_checksums"
	reIgnoreFilePath          = regexp.MustCompile(`^.*/`)
)

// Target wraps a build.proto Target message with some lazily computed
// properties.
type Target struct {
	// Name of the target (stringified bazel label)
	name string
	// If this target is a rule, the type of rule; otherwise, this is the empty
	// string
	ruleType string
	// If this target has tags, this map holds all tags
	tags map[string]struct{}
	// Hash of this target's attributes only
	shallowHash uint32
	// Memoized hash of this target, including transitive target hashes. If nil,
	// the hash is not computed yet; use getHash() to fetch a computed hash.
	hash *uint32
	// Target names of the direct dependencies for this target
	depNames []string
	// If this target is a rule with dependencies, direct dependency node pointers
	// are stored here for easy traversal.
	deps []*Target
}

type TargetHashes map[string]uint32

// Creates a Target object that holds computations based on the supplied target
// proto message.
func NewTarget(w *Workspace, t *bpb.Target) (*Target, error) {
	shallow, err := shallowHash(w, t)
	if err != nil {
		return nil, err
	}

	return &Target{
		name:        extractName(t),
		ruleType:    extractRuleType(t),
		tags:        extractTags(t),
		shallowHash: shallow,
		depNames:    extractDepNames(t),
	}, nil
}

// Creates a Target object from the supplied proto message that represents an
// external target. This target only has attributes based on the hashes used
// during download of the external repository, to save time on hashing
// third-party files that are unlikely to change often. These hashes are fetched
// from the supplied set of workspace events.
func NewExternalPseudoTarget(t *bpb.Target, eventMap map[string][]*bpb.WorkspaceEvent) (*Target, error) {
	nameCopy := extractName(t)
	lbl, err := labelFromString(nameCopy)
	if err != nil {
		return nil, err
	}
	if !lbl.isExternal() {
		return nil, fmt.Errorf("target %q is not external", nameCopy)
	}

	lbl = lbl.toCoarseExternal()
	events := eventMap[lbl.String()]

	newTarget := &bpb.Target{
		Type: bpb.Target_RULE.Enum(),
		Rule: &bpb.Rule{
			Name:      &nameCopy,
			RuleInput: extractDepNames(t),
			Attribute: []*bpb.Attribute{
				{
					Name:            &pseudoTargetAttributeName,
					Type:            bpb.Attribute_STRING_LIST.Enum(),
					StringListValue: extractChecksums(events),
				},
			},
		},
	}
	return NewTarget(nil, newTarget)
}

// Calculates a hash based on the attributes of this target only, including
// attributes and source file contents.
func shallowHash(w *Workspace, t *bpb.Target) (uint32, error) {
	h := fnv.New32()

	switch t.GetType() {
	case bpb.Target_RULE:
		attrList := t.GetRule().GetAttribute()
		// Sort the attributes by name so they are added to the hash in a stable
		// order
		sort.Slice(attrList, func(i, j int) bool { return attrList[i].GetName() < attrList[j].GetName() })
		for _, attr := range attrList {
			if attr.GetName() != "generator_location" {
				fmt.Fprintf(h, "%s=%s", attr.GetName(), attrValue(attr))
			} else {
				// Ignore the path prefix of the generator location.
				fmt.Fprintf(h, "%s=%s", attr.GetName(), reIgnoreFilePath.ReplaceAllString(attrValue(attr), ""))
			}
		}
	case bpb.Target_SOURCE_FILE:
		lbl, err := labelFromString(t.GetSourceFile().GetName())
		if err != nil {
			return 0, err
		}
		f, err := w.OpenSource(lbl.filePath())
		if err != nil {
			return 0, fmt.Errorf("can't open source file %q: %w", lbl.filePath(), err)
		}
		defer f.Close()

		err = hashFile(h, f)
		if err != nil {
			// TODO(scott): After moving to go 1.17, replace this error
			// introspection with a call to Stat before Open (requires os.DirFS to
			// return an fs.StatFS). String validation is hacky, but it works for both
			// tests and prod, which return different error types so type assertion is
			// not possible here.
			if strings.Contains(err.Error(), "is a directory") {
				// Somehow a directory got passed as a label. This sometimes happens to
				// format directories onto action commandlines; in these cases, the
				// action doesn't depend on the directory's contents per se, but rather
				// the directory name. Add the name to the hash and continue.
				fmt.Fprintf(h, "%s", extractName(t))
			} else {
				return 0, err
			}
		}

	case bpb.Target_GENERATED_FILE:
		// The hash of a generated file is based solely on the hash of the
		// generating rule, which was handled above by adding all deps to the hash.
		// No need to do anything more here.

	case bpb.Target_PACKAGE_GROUP:
		return 0, fmt.Errorf("PACKAGE_GROUP hashing not implemented")
	case bpb.Target_ENVIRONMENT_GROUP:
		return 0, fmt.Errorf("ENVIRONMENT_GROUP hashing not implemented")
	}

	return h.Sum32(), nil
}

func (t *Target) Name() string {
	return t.name
}

func (t *Target) RuleType() string {
	return t.ruleType
}

// ResolveDeps resolves each target name to the actual target object using the
// supplied mapping.
func (t *Target) ResolveDeps(others map[string]*Target) error {
	for _, dep := range t.depNames {
		other, ok := others[dep]
		if !ok {
			return fmt.Errorf("target %q has non-existent dep %q", t.name, dep)
		}
		t.deps = append(t.deps, other)
	}
	return nil
}

// Name returns the name of a Target message, which is part of a pseudo-union
// message (enum + one populated optional field).
func extractName(t *bpb.Target) string {
	switch t.GetType() {
	case bpb.Target_RULE:
		return t.GetRule().GetName()
	case bpb.Target_SOURCE_FILE:
		return t.GetSourceFile().GetName()
	case bpb.Target_GENERATED_FILE:
		return t.GetGeneratedFile().GetName()
	case bpb.Target_PACKAGE_GROUP:
		return t.GetPackageGroup().GetName()
	case bpb.Target_ENVIRONMENT_GROUP:
		return t.GetEnvironmentGroup().GetName()
	}
	// This shouldn't happen; check that all cases are covered.
	panic(fmt.Sprintf("can't get name for type %q", t.GetType()))
}

// extractRuleType returns a string representing the type of rule for the
// supplied Target proto message.
func extractRuleType(t *bpb.Target) string {
	if t.GetType() != bpb.Target_RULE {
		return ""
	}
	return t.GetRule().GetRuleClass()
}

// extractTags returns the set of tags present on supplied Target proto message.
func extractTags(t *bpb.Target) map[string]struct{} {
	tags := map[string]struct{}{}

	attrList := t.GetRule().GetAttribute()
	for _, attr := range attrList {
		if attr.GetName() != "tags" {
			continue
		}
		if attr.GetType() != bpb.Attribute_STRING_LIST {
			continue
		}
		for _, t := range attr.GetStringListValue() {
			tags[t] = struct{}{}
		}
	}
	return tags
}

// extractTags returns the set of dependencies present on supplied Target proto
// message, by stringified label.
func extractDepNames(t *bpb.Target) []string {
	switch t.GetType() {
	case bpb.Target_RULE:
		return t.GetRule().GetRuleInput()
	case bpb.Target_GENERATED_FILE:
		return []string{t.GetGeneratedFile().GetGeneratingRule()}
	}
	return nil
}

// extractChecksums returns a sorted list of download hashes from a set of
// relevant workspace events.
func extractChecksums(events []*bpb.WorkspaceEvent) []string {
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
	sort.Strings(checksums)
	return checksums
}

func (t *Target) containsTag(tag string) bool {
	_, ok := t.tags[tag]
	return ok
}

// getHash returns the computed hash from this target, recursively evaluating
// dependencies if they are not already hashed themselves.
// This hash should change if:
// * t is a source file, and the contents change (t.shallowHash should change)
// * t is a rule, and one of the following changes:
//   - Its attributes, in a meaningful way (some attributes are unordered -
//     t.shallowHash should change)
//   - A hash of one of its direct dependencies changes (getHash on an element
//     of t.deps changes)
//   - The Starlark code of the producing rule changes
func (t *Target) getHash(w *Workspace) (uint32, error) {
	if t.hash != nil {
		return *t.hash, nil
	}

	h := fnv.New32()

	for _, dep := range t.deps {
		hash, err := dep.getHash(w)
		if err != nil {
			// TODO(scott): Log this condition
		} else {
			fmt.Fprintf(h, "%d", hash)
		}
	}

	fmt.Fprintf(h, "%d", t.shallowHash)

	hash := h.Sum32()
	t.hash = &hash
	return hash, nil
}

// hashFile adds the contents of a file at path `path` to the provided
// hash.Hash.
func hashFile(h hash.Hash, f io.Reader) error {
	_, err := io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("can't read file for hashing: %v", err)
	}
	return nil
}

// attrValue returns a string representation of an attribute value. This
// transformation doesn't need to be reversible, but it does need to be
// deterministic; dicts need to be sorted before serialization (although
// ordered lists should not be).
func attrValue(attr *bpb.Attribute) string {
	switch attr.GetType() {
	case bpb.Attribute_INTEGER:
		return strconv.FormatInt(int64(attr.GetIntValue()), 10)

	case bpb.Attribute_INTEGER_LIST:
		var s []string
		for _, i := range attr.GetIntListValue() {
			s = append(s, strconv.FormatInt(int64(i), 10))
		}
		// Assume that order matters here, so don't sort the strings
		return strings.Join(s, ",")

	case bpb.Attribute_BOOLEAN:
		return strconv.FormatBool(attr.GetBooleanValue())

	case bpb.Attribute_TRISTATE:
		return attr.GetTristateValue().String()

	case bpb.Attribute_STRING,
		bpb.Attribute_LABEL,
		bpb.Attribute_OUTPUT:
		return attr.GetStringValue()

	case bpb.Attribute_STRING_LIST,
		bpb.Attribute_LABEL_LIST,
		bpb.Attribute_OUTPUT_LIST,
		bpb.Attribute_DISTRIBUTION_SET:
		val := attr.GetStringListValue()
		// Assume that order matters here, so don't sort the strings
		return strings.Join(val, ",")

	case bpb.Attribute_STRING_DICT:
		val := attr.GetStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_DICT_UNARY:
		val := attr.GetLabelDictUnaryValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_LIST_DICT:
		val := attr.GetLabelListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_KEYED_STRING_DICT:
		val := attr.GetLabelKeyedStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_STRING_LIST_DICT:
		val := attr.GetStringListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LICENSE:
		// License changes shouldn't trigger a rebuild; don't include in the hash
		return ""
	default:
		// TODO: Determine how to handle these cases
		// case bpb.Attribute_FILESET_ENTRY_LIST:
		// case bpb.Attribute_UNKNOWN:
		// case bpb.Attribute_SELECTOR_LIST:
		// case bpb.Attribute_DEPRECATED_STRING_DICT_UNARY:
		panic(fmt.Sprintf("unsupported attribute type: %v", attr.GetType()))
	}
}

// Diff returns the changed targets between this set and a baseline set.
// Ordering of the two sets is important, as targets only present in baseline
// are omitted, whereas targets only present in this set are included.
func (h TargetHashes) Diff(baseline TargetHashes) []string {
	diffs := []string{}
	for k, v := range h {
		oldVal, ok := baseline[k]
		if !ok || oldVal != v {
			diffs = append(diffs, k)
		}
	}
	return diffs
}
